//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

func (r *nativeResources) visibleRows() []resourceRow {
	if r.query == "" {
		return r.rows
	}
	return r.filtered
}
func (r *nativeResources) refilter() {
	query := strings.ToLower(strings.TrimSpace(r.query))
	r.filtered = nil
	for _, row := range r.rows {
		if strings.Contains(strings.ToLower(row.name), query) || strings.Contains(strings.ToLower(filepath.Base(row.path)), query) {
			r.filtered = append(r.filtered, row)
		}
	}
	r.offset = min(r.offset, max(0, (len(r.visibleRows())-1)/resourcePageSize*resourcePageSize))
}
func (r *nativeResources) setQuery(query string) {
	text := []rune(query)
	if len(text) > 128 {
		text = text[:128]
	}
	r.query = string(text)
	r.caret = len(text)
	r.selectAll = false
	r.caretVisible = true
	r.offset = 0
	r.refilter()
}
func (r *nativeResources) insertSearch(text string) {
	query := []rune(r.query)
	position := min(max(0, r.caret), len(query))
	if r.selectAll {
		query = nil
		position = 0
	}
	inserted := []rune(text)
	filtered := inserted[:0]
	for _, c := range inserted {
		if c >= 32 && c != 127 && c != '\uFFFD' {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) > 128-len(query) {
		filtered = filtered[:128-len(query)]
	}
	result := append([]rune(nil), query[:position]...)
	result = append(result, filtered...)
	result = append(result, query[position:]...)
	r.setQuery(string(result))
	r.caret = position + len(filtered)
}
func (n *nativeGlassPanel) focusResourceSearch() {
	if n.resources.config.AllBackground {
		return
	}
	n.focus = 25
	n.resources.caretVisible = true
	n.resources.caret = len([]rune(n.resources.query))
	user32.NewProc("SetTimer").Call(n.hwnd, 0x5254, 500, 0)
	user32.NewProc("InvalidateRect").Call(n.hwnd, 0, 0)
}
func (n *nativeGlassPanel) resourceSearchKey(key uintptr) bool {
	if !n.resourcePage || n.resources.config.AllBackground {
		return false
	}
	ctrl, _, _ := user32.NewProc("GetKeyState").Call(0x11)
	if ctrl&0x8000 != 0 && key == 'F' {
		n.focusResourceSearch()
		n.resources.selectAll = n.resources.query != ""
		return true
	}
	if n.focus != 25 {
		return false
	}
	r := &n.resources
	query := []rune(r.query)
	r.caret = min(max(0, r.caret), len(query))
	handled := true
	switch key {
	case 9:
		return false
	case 27:
		if r.query != "" {
			r.setQuery("")
		} else {
			n.focus = -1
		}
	case 13:
		n.focus = n.nextFocus(25, 1)
	case 0x24:
		r.caret = 0
		r.selectAll = false
	case 0x23:
		r.caret = len(query)
		r.selectAll = false
	case 0x25:
		if r.selectAll {
			r.caret = 0
		} else {
			r.caret = max(0, r.caret-1)
		}
		r.selectAll = false
	case 0x27:
		if r.selectAll {
			r.caret = len(query)
		} else {
			r.caret = min(len(query), r.caret+1)
		}
		r.selectAll = false
	case 0x2e:
		if r.selectAll {
			r.setQuery("")
		} else if r.caret < len(query) {
			position := r.caret
			r.setQuery(string(append(query[:position], query[position+1:]...)))
			r.caret = position
		}
	default:
		if ctrl&0x8000 != 0 {
			switch key {
			case 'A':
				r.selectAll = r.query != ""
			case 'V':
				r.insertSearch(resourceClipboardText())
			default:
				handled = false
			}
		}
	}
	r.caretVisible = true
	user32.NewProc("InvalidateRect").Call(n.hwnd, 0, 0)
	return handled
}
func (n *nativeGlassPanel) resourceSearchChar(char uintptr) bool {
	if !n.resourcePage || n.focus != 25 {
		return false
	}
	r := &n.resources
	if char == 8 {
		if r.selectAll {
			r.setQuery("")
		} else {
			query := []rune(r.query)
			position := min(max(0, r.caret), len(query))
			if position > 0 {
				r.setQuery(string(append(query[:position-1], query[position:]...)))
				r.caret = position - 1
			}
		}
	} else if char >= 32 && char <= utf8.MaxRune && !utf16.IsSurrogate(rune(char)) {
		r.insertSearch(string(rune(char)))
	}
	user32.NewProc("InvalidateRect").Call(n.hwnd, 0, 0)
	return true
}
func resourceClipboardText() string {
	ok, _, _ := user32.NewProc("OpenClipboard").Call(0)
	if ok == 0 {
		return ""
	}
	defer user32.NewProc("CloseClipboard").Call()
	handle, _, _ := user32.NewProc("GetClipboardData").Call(13)
	if handle == 0 {
		return ""
	}
	size, _, _ := kernel32.NewProc("GlobalSize").Call(handle)
	if size < 2 {
		return ""
	}
	ptr, _, _ := kernel32.NewProc("GlobalLock").Call(handle)
	if ptr == 0 {
		return ""
	}
	defer kernel32.NewProc("GlobalUnlock").Call(handle)
	data := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), min(size/2, 512))
	return syscall.UTF16ToString(data)
}
func (n *nativeGlassPanel) drawResourceSearch(ink, muted, card uint32) {
	var saved uint32
	nativeG("GdipSaveGraphics", n.graphics, uintptr(unsafe.Pointer(&saved)))
	defer nativeG("GdipRestoreGraphics", n.graphics, uintptr(saved))
	nativeG("GdipTranslateWorldTransform", n.graphics, nativeF(0), nativeF(102), 0)
	r := &n.resources
	focused := n.focus == 25
	n.round(20, 78, 318, 29, 10, card, false)
	if focused {
		n.roundFloat(20.5, 78.5, 317, 28, 10, 0xff669dff, true, 1)
	}
	n.round(31, 86, 10, 10, 5, muted, true)
	n.line(40, 95, 44, 99, muted, 1.5)
	text, color := r.query, ink
	if text == "" && !focused {
		text = "Cari aplikasi…"
		color = muted
	}
	// Scroll the displayed text to keep the caret visible for long queries.
	runes := []rune(text)
	start := 0
	if focused && r.caret > 30 {
		start = r.caret - 30
	}
	if start > len(runes) {
		start = 0
	}
	text = string(runes[start:])
	if focused && r.selectAll && r.query != "" {
		n.round(54, 84, min(249, n.resourceSearchWidth(text)), 17, 2, 0x993f83f8, false)
	}
	n.drawText(text, 52, 84, 251, 21, 12, false, color, false, true)
	if focused && r.caretVisible && !r.selectAll {
		before := []rune(r.query)
		position := min(max(0, r.caret), len(before))
		prefix := syscall.StringToUTF16(string(before[min(start, position):position]))
		bounds := nativeFloatRect{0, 0, 2000, 25}
		var measured nativeFloatRect
		nativeG("GdipMeasureString", n.graphics, uintptr(unsafe.Pointer(&prefix[0])), uintptr(len(prefix)-1), n.fonts[24], uintptr(unsafe.Pointer(&bounds)), 0, uintptr(unsafe.Pointer(&measured)), 0, 0)
		width := n.textWidthLogical(int(measured.w) - 3)
		if position == start {
			width = 0
		}
		x := min(301, 52+max(0, width))
		n.line(x, 85, x, 99, ink, 1)
	}
	if r.query != "" {
		n.line(317, 88, 325, 96, muted, 1.5)
		n.line(317, 96, 325, 88, muted, 1.5)
	}
}

func (n *nativeGlassPanel) resourceSearchWidth(text string) int {
	if text == "" {
		return 0
	}
	chars := syscall.StringToUTF16(text)
	bounds := nativeFloatRect{0, 0, 4000, 25}
	var measured nativeFloatRect
	nativeG("GdipMeasureString", n.graphics, uintptr(unsafe.Pointer(&chars[0])), uintptr(len(chars)-1), n.fonts[24], uintptr(unsafe.Pointer(&bounds)), 0, uintptr(unsafe.Pointer(&measured)), 0, 0)
	return n.textWidthLogical(max(1, int(measured.w+.5)-4))
}
