//go:build windows

package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// Keep native popup-menu navigation, dismissal, and MSAA names; own only drawing.
type msaaMenuInfo struct {
	signature, length uint32
	text              *uint16
}
type menuRow struct {
	access              msaaMenuInfo // Must be first: Windows reads MSAAMENUINFO here.
	name                []uint16
	id                  uint32
	label, detail, kind string
	active, disabled    bool
}
type measureMenuItem struct {
	controlType, controlID, itemID, width, height uint32
	data                                          uintptr
}
type drawMenuItem struct {
	controlType, controlID, itemID, action, state uint32
	window, dc                                    uintptr
	bounds                                        rect
	data                                          uintptr
}
type nativeMenuInfo struct {
	size, mask, style, maxHeight uint32
	background                   uintptr
	helpID                       uint32
	data                         uintptr
}

var (
	menuFill       = user32.NewProc("FillRect")
	menuText       = user32.NewProc("DrawTextW")
	menuSetInfo    = user32.NewProc("SetMenuInfo")
	menuDrawIcon   = user32.NewProc("DrawIconEx")
	menuBrush      = gdi32.NewProc("CreateSolidBrush")
	menuFont       = gdi32.NewProc("CreateFontW")
	menuSelect     = gdi32.NewProc("SelectObject")
	menuStock      = gdi32.NewProc("GetStockObject")
	menuTextColor  = gdi32.NewProc("SetTextColor")
	menuBkMode     = gdi32.NewProc("SetBkMode")
	menuRound      = gdi32.NewProc("RoundRect")
	menuPen        = gdi32.NewProc("CreatePen")
	menuMove       = gdi32.NewProc("MoveToEx")
	menuLine       = gdi32.NewProc("LineTo")
	menuRows       = map[uint32]*menuRow{}
	menuPaintCount int
	menuIsOpen     bool
	menuDPI        int32 = 96
	menuBrandIcon  uintptr
)

func menuColor(rgb uint32) uintptr { return uintptr((rgb&255)<<16 | (rgb & 0xff00) | (rgb>>16)&255) }
func menuPx(v int32) int32         { return (v*menuDPI + 48) / 96 }
func menuRowHeight(row *menuRow) int32 {
	switch row.kind {
	case "brand":
		return 76
	case "divider":
		return 12
	case "style":
		return 52
	default:
		if row.detail != "" {
			return 52
		}
		return 42
	}
}

func makeMenuRows(cfg *config) []*menuRow {
	updateLabel, updateDetail, updateEnabled, _ := updatePresentation(updatePhase, updateAvailable, buildVersion)
	return []*menuRow{
		{id: 600, label: "Senja", detail: "Tampilan taskbar", kind: "brand", disabled: true},
		{id: cmdStyleGlass, label: "Kaca", detail: "Transparan dengan blur lembut", kind: "style", active: cfg.style == styleGlass},
		{id: cmdStyleAcrylic, label: "Akrilik", detail: "Blur lebih pekat", kind: "style", active: cfg.style == styleAcrylic},
		{id: cmdStyleClear, label: "Bening", detail: "Transparan sepenuhnya", kind: "style", active: cfg.style == styleClear},
		{id: cmdStyleSolid, label: "Solid", detail: "Gelap netral tanpa blur", kind: "style", active: cfg.style == styleSolid},
		{id: 601, kind: "divider", disabled: true},
		{id: cmdRounded, label: "Sudut membulat", kind: "toggle", active: cfg.rounded},
		{id: cmdStartup, label: "Mulai bersama Windows", kind: "toggle", active: cfg.startup},
		{id: cmdRefresh, label: "Segarkan taskbar"},
		{id: 602, kind: "divider", disabled: true},
		{id: cmdCheckUpdates, label: updateLabel, detail: updateDetail, disabled: !updateEnabled},
		{id: cmdExit, label: "Keluar dari Senja", kind: "exit"},
	}
}

func paintMenuFill(dc uintptr, bounds rect, color uint32, radius int32) {
	brush, _, _ := menuBrush.Call(menuColor(color))
	if brush == 0 {
		return
	}
	defer deleteObject.Call(brush)
	if radius == 0 {
		menuFill.Call(dc, uintptr(unsafe.Pointer(&bounds)), brush)
		return
	}
	oldBrush, _, _ := menuSelect.Call(dc, brush)
	nullPen, _, _ := menuStock.Call(8)
	oldPen, _, _ := menuSelect.Call(dc, nullPen)
	menuRound.Call(dc, uintptr(bounds.left), uintptr(bounds.top), uintptr(bounds.right), uintptr(bounds.bottom), uintptr(radius), uintptr(radius))
	menuSelect.Call(dc, oldPen)
	menuSelect.Call(dc, oldBrush)
}

func paintMenuText(dc uintptr, value string, bounds rect, size int32, weight uint32, color uint32) {
	font, _, _ := menuFont.Call(uintptr(-menuPx(size)), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(utf16ptr("Segoe UI Variable Text"))))
	old, _, _ := menuSelect.Call(dc, font)
	menuBkMode.Call(dc, 1)
	menuTextColor.Call(dc, menuColor(color))
	menuText.Call(dc, uintptr(unsafe.Pointer(utf16ptr(value))), ^uintptr(0), uintptr(unsafe.Pointer(&bounds)), 0x20|0x4|0x800|0x8000)
	menuSelect.Call(dc, old)
	deleteObject.Call(font)
}

func paintMenuOutline(dc uintptr, bounds rect, color uint32, radius int32) {
	pen, _, _ := menuPen.Call(0, uintptr(menuPx(1)), menuColor(color))
	oldPen, _, _ := menuSelect.Call(dc, pen)
	emptyBrush, _, _ := menuStock.Call(5)
	oldBrush, _, _ := menuSelect.Call(dc, emptyBrush)
	menuRound.Call(dc, uintptr(bounds.left), uintptr(bounds.top), uintptr(bounds.right), uintptr(bounds.bottom), uintptr(radius), uintptr(radius))
	menuSelect.Call(dc, oldBrush)
	menuSelect.Call(dc, oldPen)
	deleteObject.Call(pen)
}

func paintMenuCheck(dc uintptr, x, y int32) {
	pen, _, _ := menuPen.Call(0, uintptr(menuPx(2)), menuColor(0xffffff))
	old, _, _ := menuSelect.Call(dc, pen)
	menuMove.Call(dc, uintptr(x), uintptr(y), 0)
	menuLine.Call(dc, uintptr(x+menuPx(4)), uintptr(y+menuPx(4)))
	menuLine.Call(dc, uintptr(x+menuPx(11)), uintptr(y-menuPx(4)))
	menuSelect.Call(dc, old)
	deleteObject.Call(pen)
}

func paintThemedMenuRow(dc uintptr, row *menuRow, bounds rect, selected bool) {
	paintMenuBackdrop(dc, bounds)
	l, t, r, b := bounds.left, bounds.top, bounds.right, bounds.bottom
	if row.kind == "divider" {
		y := (t + b) / 2
		paintMenuFill(dc, rect{l + menuPx(16), y, r - menuPx(16), y + menuPx(1)}, 0xb6c9dc, 0)
		return
	}
	if row.kind == "brand" {
		if menuBrandIcon != 0 {
			menuDrawIcon.Call(dc, uintptr(l+menuPx(20)), uintptr(t+menuPx(22)), menuBrandIcon, uintptr(menuPx(30)), uintptr(menuPx(30)), 0, 0, 3)
		}
		paintMenuText(dc, "Senja", rect{l + menuPx(64), t + menuPx(15), r - menuPx(16), t + menuPx(40)}, 20, 600, 0x0f172a)
		paintMenuText(dc, row.detail, rect{l + menuPx(64), t + menuPx(40), r - menuPx(16), t + menuPx(59)}, 12, 400, 0x334a64)
		return
	}
	if row.active && row.kind == "style" {
		paintMenuFill(dc, rect{l + menuPx(8), t + menuPx(2), r - menuPx(8), b - menuPx(2)}, 0x2563eb, menuPx(24))
	} else if selected && !row.disabled {
		paintMenuFill(dc, rect{l + menuPx(8), t + menuPx(2), r - menuPx(8), b - menuPx(2)}, 0xe5f0fa, menuPx(24))
	}
	color := uint32(0x172b45)
	if row.active && row.kind == "style" {
		color = 0xffffff
	}
	if row.disabled {
		color = 0x465c74
	}
	left := l + menuPx(20)
	textRight := r - menuPx(20)
	if row.kind == "toggle" || row.kind == "style" {
		textRight = r - menuPx(58)
	}
	if row.detail == "" {
		paintMenuText(dc, row.label, rect{left, t, textRight, b}, 14, 400, color)
	} else {
		paintMenuText(dc, row.label, rect{left, t + menuPx(7), textRight, t + menuPx(27)}, 14, 600, color)
		detailColor := uint32(0x334a64)
		if row.active {
			detailColor = 0xe6efff
		}
		paintMenuText(dc, row.detail, rect{left, t + menuPx(28), textRight, b - menuPx(5)}, 12, 400, detailColor)
	}
	if row.kind == "style" && row.active {
		paintMenuCheck(dc, r-menuPx(37), (t+b)/2)
	}
	if row.kind == "toggle" {
		x, y := r-menuPx(51), (t+b-menuPx(18))/2
		fill, dot := uint32(0x8196ac), uint32(0xffffff)
		if row.active {
			fill, dot = 0x2563eb, 0xffffff
		}
		paintMenuFill(dc, rect{x, y, x + menuPx(32), y + menuPx(18)}, fill, menuPx(18))
		dotX := x + menuPx(3)
		if row.active {
			dotX = x + menuPx(17)
		}
		paintMenuFill(dc, rect{dotX, y + menuPx(3), dotX + menuPx(12), y + menuPx(15)}, dot, menuPx(12))
	}
}

func measureThemedMenu(lParam uintptr) bool {
	if lParam == 0 {
		return false
	}
	item := (*measureMenuItem)(unsafe.Pointer(lParam))
	row := menuRows[item.itemID]
	if item.controlType != 1 || row == nil {
		return false
	}
	item.width, item.height = uint32(menuPx(288)), uint32(menuPx(menuRowHeight(row)))
	return true
}
func drawThemedMenu(lParam uintptr) bool {
	if lParam == 0 {
		return false
	}
	item := (*drawMenuItem)(unsafe.Pointer(lParam))
	row := menuRows[item.itemID]
	if item.controlType != 1 || row == nil {
		return false
	}
	paintThemedMenuRow(item.dc, row, item.bounds, item.state&1 != 0)
	menuPaintCount++
	return true
}

func showThemedTrayMenu(hwnd syscall.Handle, cfg *config) {
	if menuIsOpen {
		return
	}
	menuIsOpen = true
	defer func() { menuIsOpen = false }()
	menu, _, _ := createPopupMenu.Call()
	if menu == 0 {
		return
	}
	defer destroyMenu.Call(menu)
	menuDPI = 96
	if getDpiForWindow.Find() == nil {
		taskbar, _, _ := findWindowW.Call(uintptr(unsafe.Pointer(utf16ptr("Shell_TrayWnd"))), 0)
		if dpi, _, _ := getDpiForWindow.Call(taskbar); dpi != 0 {
			menuDPI = int32(dpi)
		}
	}
	brush, _, _ := menuBrush.Call(menuColor(0xd4e4f1))
	defer deleteObject.Call(brush)
	info := nativeMenuInfo{size: uint32(unsafe.Sizeof(nativeMenuInfo{})), mask: 2 | 0x10, style: 0x80000000, background: brush}
	menuSetInfo.Call(menu, uintptr(unsafe.Pointer(&info)))
	exe, _ := syscall.UTF16PtrFromString(iconFilePath())
	menuBrandIcon, _, _ = loadImageW.Call(0, uintptr(unsafe.Pointer(exe)), imageIcon, uintptr(menuPx(32)), uintptr(menuPx(32)), lrLoadFromFile)
	defer func() {
		if menuBrandIcon != 0 {
			user32.NewProc("DestroyIcon").Call(menuBrandIcon)
		}
		menuBrandIcon = 0
		menuRows = map[uint32]*menuRow{}
	}()
	rows := makeMenuRows(cfg)
	menuPaintCount = 0
	for _, row := range rows {
		row.name, _ = syscall.UTF16FromString(row.label)
		row.access = msaaMenuInfo{signature: 0xaa0df00d, length: uint32(len(row.name) - 1), text: &row.name[0]}
		menuRows[row.id] = row
		flags := uintptr(0x100) // MF_OWNERDRAW
		if row.disabled {
			flags |= 0x2
		}
		if row.active {
			flags |= mfChecked
		}
		ok, _, _ := appendMenuW.Call(menu, flags, uintptr(row.id), uintptr(unsafe.Pointer(&row.access)))
		if ok == 0 {
			return
		}
	}
	var cursor point
	getCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
	if menuPreviewPoint != nil {
		cursor = *menuPreviewPoint
	}
	var totalHeight int32
	for _, row := range rows {
		totalHeight += menuRowHeight(row)
	}
	prepareMenuBackdrop(cursor, totalHeight)
	setForegroundWindow.Call(uintptr(hwnd))
	stopGlass := startMenuGlass()
	defer stopGlass()
	logEvent("INFO", "menu.open", "glass native menu, Indonesian labels and MSAA names")
	selected, _, _ := trackPopupMenuEx.Call(menu, tpmRetCmd|tpmNoNotify|tpmNoAnimation, uintptr(cursor.x), uintptr(cursor.y), uintptr(hwnd), 0)
	postMessageW.Call(uintptr(hwnd), wmNull, 0, 0)
	runtime.KeepAlive(rows)
	logEvent("INFO", "menu.close", fmt.Sprintf("selected=%d painted_rows=%d", selected, menuPaintCount))
	if selected != 0 && !materialPreview {
		selectCommand(hwnd, uint32(selected), cfg)
	}
}
