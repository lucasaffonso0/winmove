package main

import (
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/xproto"
)

type Monitor struct {
	width, height, offsetX, offsetY int
}

func getActiveWindow(X *xgb.Conn, root xproto.Window) (xproto.Window, error) {
	atom, err := xproto.InternAtom(X, true, uint16(len("_NET_ACTIVE_WINDOW")), "_NET_ACTIVE_WINDOW").Reply()
	if err != nil {
		return 0, err
	}

	prop, err := xproto.GetProperty(X, false, root, atom.Atom, xproto.AtomWindow, 0, 1).Reply()
	if err != nil {
		return 0, err
	}

	if len(prop.Value) < 4 {
		return 0, fmt.Errorf("nenhuma janela ativa")
	}

	win := xproto.Window(
		uint32(prop.Value[0]) |
			uint32(prop.Value[1])<<8 |
			uint32(prop.Value[2])<<16 |
			uint32(prop.Value[3])<<24)

	return win, nil
}

func getWindowGeometry(X *xgb.Conn, win xproto.Window) (x, width int, err error) {
	geom, err := xproto.GetGeometry(X, xproto.Drawable(win)).Reply()
	if err != nil {
		return 0, 0, err
	}
	return int(geom.X), int(geom.Width), nil
}

func getMonitors(X *xgb.Conn, root xproto.Window) ([]Monitor, error) {
	if err := randr.Init(X); err != nil {
		return nil, err
	}

	resources, err := randr.GetScreenResourcesCurrent(X, root).Reply()
	if err != nil {
		return nil, err
	}

	var monitors []Monitor
	for _, output := range resources.Outputs {
		outInfo, err := randr.GetOutputInfo(X, output, 0).Reply()
		if err != nil || outInfo.Connection != randr.ConnectionConnected || len(outInfo.Crtcs) == 0 {
			continue
		}

		crtcInfo, err := randr.GetCrtcInfo(X, outInfo.Crtc, 0).Reply()
		if err != nil || crtcInfo.Width == 0 || crtcInfo.Height == 0 {
			continue
		}

		monitors = append(monitors, Monitor{
			width:   int(crtcInfo.Width),
			height:  int(crtcInfo.Height),
			offsetX: int(crtcInfo.X),
			offsetY: int(crtcInfo.Y),
		})
	}

	sort.Slice(monitors, func(i, j int) bool {
		return monitors[i].offsetX < monitors[j].offsetX
	})

	return monitors, nil
}

func removeMaximize(X *xgb.Conn, win xproto.Window, root xproto.Window) error {
	// Remove maximized_vert and maximized_horz properties

	atomWmState, err := xproto.InternAtom(X, false, uint16(len("_NET_WM_STATE")), "_NET_WM_STATE").Reply()
	if err != nil {
		return err
	}

	atomMaxVert, err := xproto.InternAtom(X, false, uint16(len("_NET_WM_STATE_MAXIMIZED_VERT")), "_NET_WM_STATE_MAXIMIZED_VERT").Reply()
	if err != nil {
		return err
	}

	atomMaxHorz, err := xproto.InternAtom(X, false, uint16(len("_NET_WM_STATE_MAXIMIZED_HORZ")), "_NET_WM_STATE_MAXIMIZED_HORZ").Reply()
	if err != nil {
		return err
	}

	// Remove maximized vertically
	dataVert := []uint32{
		0, // _NET_WM_STATE_REMOVE
		uint32(atomMaxVert.Atom),
		0,
		1, // source indication
		0,
	}
	evVert := xproto.ClientMessageEvent{
		Format: 32,
		Window: win,
		Type:   atomWmState.Atom,
		Data:   xproto.ClientMessageDataUnionData32New(dataVert),
	}

	err = xproto.SendEventChecked(
		X,
		false,
		root,
		xproto.EventMaskSubstructureRedirect|xproto.EventMaskSubstructureNotify,
		string(evVert.Bytes()),
	).Check()
	if err != nil {
		return err
	}

	// Remove maximized horizontally
	dataHorz := []uint32{
		0, // _NET_WM_STATE_REMOVE
		uint32(atomMaxHorz.Atom),
		0,
		1, // source indication
		0,
	}
	evHorz := xproto.ClientMessageEvent{
		Format: 32,
		Window: win,
		Type:   atomWmState.Atom,
		Data:   xproto.ClientMessageDataUnionData32New(dataHorz),
	}

	err = xproto.SendEventChecked(
		X,
		false,
		root,
		xproto.EventMaskSubstructureRedirect|xproto.EventMaskSubstructureNotify,
		string(evHorz.Bytes()),
	).Check()

	return err
}

func moveResizeWindow(X *xgb.Conn, win xproto.Window, x, y, width, height int) error {
	mask := uint16(xproto.ConfigWindowX | xproto.ConfigWindowY | xproto.ConfigWindowWidth | xproto.ConfigWindowHeight)
	values := []uint32{uint32(x), uint32(y), uint32(width), uint32(height)}
	return xproto.ConfigureWindowChecked(X, win, mask, values).Check()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Uso: go run main.go left|right")
		return
	}

	direction := os.Args[1]
	if direction != "left" && direction != "right" {
		fmt.Println("Direção deve ser 'left' ou 'right'")
		return
	}

	X, err := xgb.NewConn()
	if err != nil {
		log.Fatalf("Erro ao conectar ao X: %v\n", err)
	}
	defer X.Close()

	setup := xproto.Setup(X)
	root := setup.DefaultScreen(X).Root

	win, err := getActiveWindow(X, root)
	if err != nil {
		log.Fatalf("Nenhuma janela ativa detectada: %v\n", err)
	}

	x, width, err := getWindowGeometry(X, win)
	if err != nil {
		log.Fatalf("Erro ao obter geometria da janela: %v\n", err)
	}

	monitors, err := getMonitors(X, root)
	if err != nil || len(monitors) == 0 {
		log.Fatalf("Erro ao pegar monitores: %v\n", err)
	}

	winCenterX := x + width/2
	currentIndex := 0
	for i, m := range monitors {
		if winCenterX >= m.offsetX && winCenterX < m.offsetX+m.width {
			currentIndex = i
			break
		}
	}

	moveToOther := false
	windowRight := x + width

	if direction == "left" && x <= monitors[currentIndex].offsetX+5 {
		currentIndex--
		moveToOther = true
	} else if direction == "right" && windowRight >= monitors[currentIndex].offsetX+monitors[currentIndex].width-5 {
		currentIndex++
		moveToOther = true
	}

	if currentIndex < 0 {
		currentIndex = 0
		moveToOther = false
	}
	if currentIndex >= len(monitors) {
		currentIndex = len(monitors) - 1
		moveToOther = false
	}

	mon := monitors[currentIndex]
	halfWidth := mon.width / 2
	screenHeight := mon.height

	var newX int
	if moveToOther {
		if direction == "left" {
			newX = mon.offsetX + mon.width - halfWidth
		} else {
			newX = mon.offsetX
		}
	} else {
		if direction == "left" {
			newX = mon.offsetX
		} else {
			newX = mon.offsetX + mon.width - halfWidth
		}
	}

	err = removeMaximize(X, win, root)
	if err != nil {
		log.Printf("Aviso: não foi possível remover maximizado: %v\n", err)
	}

	err = moveResizeWindow(X, win, newX, 0, halfWidth, screenHeight)
	if err != nil {
		log.Fatalf("Erro ao mover/redimensionar janela: %v\n", err)
	}
}
