//go:build !webui

package wailsruntime

import "github.com/wailsapp/wails/v3/pkg/application"

type OpenDialogOptions struct {
	Title            string
	DefaultDirectory string
	Filters          []FileFilter
}

type SaveDialogOptions struct {
	Title            string
	DefaultDirectory string
	DefaultFilename  string
	Filters          []FileFilter
}

type FileFilter struct {
	DisplayName string
	Pattern     string
}

var app *application.App
var window *application.WebviewWindow

func SetApplication(a *application.App, w *application.WebviewWindow) { app, window = a, w }
func Quit(_ any) {
	if app != nil {
		app.Quit()
	}
}
func WindowShow(_ any) {
	if window != nil {
		window.Show()
	}
}
func WindowHide(_ any) {
	if window != nil {
		window.Hide()
	}
}

func OpenDirectoryDialog(_ any, o OpenDialogOptions) (string, error) {
	if app == nil {
		return "", nil
	}
	return app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title: o.Title, Directory: o.DefaultDirectory, CanChooseDirectories: true, Window: window,
	}).PromptForSingleSelection()
}
func OpenFileDialog(_ any, o OpenDialogOptions) (string, error) {
	if app == nil {
		return "", nil
	}
	filters := make([]application.FileFilter, len(o.Filters))
	for i, f := range o.Filters {
		filters[i] = application.FileFilter{DisplayName: f.DisplayName, Pattern: f.Pattern}
	}
	return app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title: o.Title, Directory: o.DefaultDirectory, Filters: filters, Window: window,
	}).PromptForSingleSelection()
}
func SaveFileDialog(_ any, o SaveDialogOptions) (string, error) {
	if app == nil {
		return "", nil
	}
	filters := make([]application.FileFilter, len(o.Filters))
	for i, f := range o.Filters {
		filters[i] = application.FileFilter{DisplayName: f.DisplayName, Pattern: f.Pattern}
	}
	return app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title: o.Title, Directory: o.DefaultDirectory, Filename: o.DefaultFilename, Filters: filters, Window: window,
	}).PromptForSingleSelection()
}
func EventsEmit(_ any, name string, data ...any) {
	if app != nil {
		app.Event.Emit(name, data...)
	}
}
