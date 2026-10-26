//go:build windows

package main

const (
	wmDestroy             = 0x0002
	wmClose               = 0x0010
	wmCommand             = 0x0111
	wmApp                 = 0x8000
	wmTray                = wmApp + 1
	wmShowMenu            = wmApp + 2
	wmBeginUpdate         = wmApp + 3
	wmUpdateResult        = wmApp + 4
	wmUpdatePhase         = wmApp + 61
	wmUpdateQuery         = wmApp + 62
	wmUpdateWorkerDone    = wmApp + 63
	wmUpdateProgress      = wmApp + 67
	wmUpdateProgressQuery = wmApp + 68
	wmUpdateElapsedQuery  = wmApp + 69
	wmNull                = 0x0000
	wmRButtonUp           = 0x0205
	wmLButtonUp           = 0x0202
	wmLButtonDbl          = 0x0203

	wsExToolWindow = 0x00000080
	wsExNoActivate = 0x08000000
	wsPopup        = 0x80000000

	csHRedraw = 0x0002
	csVRedraw = 0x0001

	iconApplication = 32512
	cursorArrow     = 32512
	imageIcon       = 1
	lrLoadFromFile  = 0x00000010
	lrDefaultColor  = 0x00000000
	smCxSmIcon      = 49
	smCySmIcon      = 50

	nifMessage         = 0x00000001
	nifIcon            = 0x00000002
	nifTip             = 0x00000004
	nimAdd             = 0x00000000
	nimModify          = 0x00000001
	nimDelete          = 0x00000002
	nimSetVersion      = 0x00000004
	notifyIconVersion4 = 4

	mfString    = 0x00000000
	mfSeparator = 0x00000800
	mfPopup     = 0x00000010
	mfChecked   = 0x00000008

	tpmLeftButton  = 0x0000
	tpmRetCmd      = 0x0100
	tpmNoNotify    = 0x0080
	tpmNoAnimation = 0x4000
	tpmRightButton = 0x0002

	messageBoxError = 0x00000010
	messageBoxInfo  = 0x00000040

	hkeyCurrentUser      = 0x80000001
	keyRead              = 0x20019
	keyWrite             = 0x20006
	regSz                = 1
	errorSuccess         = 0
	errorFileNotFound    = 2
	errorAlreadyExists   = 183
	errorClassAlreadyReg = 1410

	appUserModelID = "Senja.Taskbar"
	appName        = "Senja"
	configDirName  = "Senja"
	configFileName = "settings.ini"
	logFileName    = "senja.log"
	mutexName      = "Local\\Senja.SingleInstance"
	windowClass    = "SenjaHostWindow"
	taskbarMessage = "TaskbarCreated"

	cmdStyleGlass   = 100
	cmdStyleAcrylic = 101
	cmdStyleClear   = 102
	cmdStyleSolid   = 103
	cmdRounded      = 105
	cmdStartup      = 110
	cmdRefresh      = 111
	cmdCheckUpdates = 112
	cmdAutoUpdates  = 113
	cmdExit         = 140
)

const (
	styleNormal = iota
	styleGlass
	styleAcrylic
	styleClear
	styleSolid
)
