//go:build !webui

package app

import "github.com/wailsapp/wails/v3/pkg/application"

var desktopWailsApp *application.App

func wailsQuit(_ any) {
	if desktopWailsApp != nil {
		desktopWailsApp.Quit()
	}
}

func wailsWindowShow(_ any) {
	if desktopWindow != nil {
		desktopWindow.Show()
	}
}

func wailsWindowHide(_ any) {
	if desktopWindow != nil {
		desktopWindow.Hide()
	}
}

var desktopWindow *application.WebviewWindow

type wailsFileFilter = application.FileFilter
type wailsOpenDialogOptions = application.OpenFileDialogOptions
type wailsSaveDialogOptions = application.SaveFileDialogOptions

func wailsOpenDirectoryDialog(_ any, options wailsOpenDialogOptions) (string, error) {
	if desktopWailsApp == nil {
		return "", nil
	}
	options.CanChooseDirectories = true
	options.CanChooseFiles = false
	options.Window = desktopWindow
	return desktopWailsApp.Dialog.OpenFileWithOptions(&options).PromptForSingleSelection()
}

func wailsOpenFileDialog(_ any, options wailsOpenDialogOptions) (string, error) {
	if desktopWailsApp == nil {
		return "", nil
	}
	options.Window = desktopWindow
	return desktopWailsApp.Dialog.OpenFileWithOptions(&options).PromptForSingleSelection()
}

func wailsSaveFileDialog(_ any, options wailsSaveDialogOptions) (string, error) {
	if desktopWailsApp == nil {
		return "", nil
	}
	options.Window = desktopWindow
	return desktopWailsApp.Dialog.SaveFileWithOptions(&options).PromptForSingleSelection()
}
