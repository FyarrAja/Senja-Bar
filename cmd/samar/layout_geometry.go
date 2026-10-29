//go:build windows

package main

// Geometry expands available space without stretching native text or increasing
// rendering resolution. Logical control coordinates remain stable for keyboard
// navigation, while pointer input uses the exact inverse projection.
const nativePanelWidth = 620

func (n *nativeGlassPanel) wide() bool { return n.w == menuPx(nativePanelWidth) }
func (n *nativeGlassPanel) sidebarWidth() int {
	if n.wide() {
		return 140
	}
	return nativeSidebar
}
func (n *nativeGlassPanel) projectX(x float32) float32 {
	if !n.projected || !n.wide() {
		return x
	}
	if x < 0 {
		return x * 1.25
	}
	return x * 4 / 3
}
func (n *nativeGlassPanel) logicalPointerX(x int) int {
	x -= n.sidebarWidth()
	if !n.wide() {
		return x
	}
	if x < 0 {
		return x * 4 / 5
	}
	return x * 3 / 4
}
func (n *nativeGlassPanel) textWidthLogical(x int) int {
	if n.projected && n.wide() {
		return x * 3 / 4
	}
	return x
}
