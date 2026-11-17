package main

import (
	"log"
	"strings"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/extension/i18n"
)

const (
	surfaceID = "main"
	searchKey = "search"
	vaultKey  = "vault"

	onQuery       extension.Handler = "query"
	onFind        extension.Handler = "find"
	onNew         extension.Handler = "new"
	onMode        extension.Handler = "mode"
	onSplit       extension.Handler = "split"
	onDraft       extension.Handler = "draft"
	onSave        extension.Handler = "save"
	onDelete      extension.Handler = "delete"
	onDeleteYes   extension.Handler = "delete.confirm"
	onDeleteNo    extension.Handler = "delete.cancel"
	onLink        extension.Handler = "link"
	onVaultInput  extension.Handler = "vault.input"
	onVaultOpen   extension.Handler = "vault.open"
	onVaultChange extension.Handler = "vault.change"
	onVaultCancel extension.Handler = "vault.cancel"
	// openPrefix starts the handler of a note's row; the rest is its path.
	openPrefix = "open:"
)

// app is the whole state of the screen. Only the goroutine reading lich's
// messages touches it.
type app struct {
	send    func(extension.Message)
	cat     *i18n.Bundle
	lang    *i18n.Language
	dataDir string

	vault string
	// picking is the vault form: the first run, or Change vault.
	picking    bool
	vaultInput string
	vaultError string

	notes    []Note
	query    string
	selected string
	// text is the open note as the person has it; saved is what the file
	// holds, so the two differ while there is something to save.
	text             string
	saved            string
	mode             mode
	confirmingDelete bool
	// split is the share of the editor in split mode.
	split float64

	// focus is the key of the field to hand the keyboard once the next tree
	// is out: lich drops a focus for a field it has not drawn yet.
	focus string
}

func (a *app) t(key string, args ...i18n.Args) string {
	return a.cat.In(a.lang.Tag(), "notes."+key, args...)
}

func (a *app) handle(msg extension.Message) {
	switch d := msg.Data.(type) {
	case extension.SurfaceData:
		a.lang.Set(d.Language)
		if msg.Kind == extension.SurfaceOpened.Name {
			a.reload()
			a.render()
		}
	case extension.LanguageData:
		a.lang.Set(d.Tag)
		a.render()
	case extension.EventData:
		a.event(d)
		a.render()
		if a.focus != "" {
			a.send(extension.Focus.New(extension.FocusData{Surface: surfaceID, Key: a.focus}))
			a.focus = ""
		}
	}
}

func (a *app) event(e extension.EventData) {
	if path, ok := strings.CutPrefix(string(e.Handler), openPrefix); ok {
		a.open(path)
		return
	}
	switch e.Handler {
	case onQuery:
		a.query = e.Text
	case onFind:
		a.focus = searchKey
	case onNew:
		a.newNote()
	case onMode:
		a.setMode(mode(e.Text))
	case onSplit:
		a.split = e.Ratio
	case onDraft:
		a.text = e.Text
	case onSave:
		// Saving ends an edit; a split keeps both sides up.
		if a.save() && a.mode == modeEdit {
			a.mode = modePreview
		}
	case onDelete:
		a.confirmingDelete = a.selected != ""
	case onDeleteYes:
		if a.confirmingDelete {
			a.deleteNote()
		}
	case onDeleteNo:
		a.confirmingDelete = false
	case onLink:
		// lich opens only http and https, and drops the rest.
		a.send(extension.OpenExternal.New(extension.OpenExternalData{URL: e.Text}))
	case onVaultInput:
		a.vaultInput, a.vaultError = e.Text, ""
	case onVaultOpen:
		a.chooseVault()
	case onVaultChange:
		a.picking, a.vaultInput, a.vaultError, a.focus = true, a.vault, "", vaultKey
	case onVaultCancel:
		a.picking = false
	}
}

func (a *app) deleteNote() {
	if err := DeleteNote(a.vault, a.selected); err != nil {
		a.confirmingDelete = false
		a.fail("error.delete", err)
		return
	}
	a.selected, a.text, a.saved, a.confirmingDelete = "", "", "", false
	a.reload()
}

// reload lists the vault again: notes change on disk behind the screen.
func (a *app) reload() {
	if a.vault == "" {
		return
	}
	notes, err := ListNotes(a.vault)
	if err != nil {
		a.fail("error.list", err)
		return
	}
	a.notes = notes
}

func (a *app) chooseVault() {
	vault, err := ResolveVault(a.vaultInput)
	if err != nil {
		a.vaultError = a.t("vault.notFolder")
		return
	}
	if !a.save() {
		return
	}
	if err := SaveSettings(a.dataDir, Settings{Vault: vault}); err != nil {
		a.fail("error.settings", err)
		return
	}
	a.vault, a.picking = vault, false
	a.selected, a.text, a.saved, a.query = "", "", "", ""
	a.reload()
}

// open shows another note. What was typed into the one left is saved first:
// there is no other copy of it.
func (a *app) open(path string) {
	if path == a.selected || !a.save() {
		return
	}
	text, err := ReadNote(a.vault, path)
	if err != nil {
		a.fail("error.read", err)
		return
	}
	a.selected, a.text, a.saved = path, text, text
	if a.mode == modeEdit {
		a.mode = modePreview
	}
}

func (a *app) newNote() {
	if a.vault == "" || !a.save() {
		return
	}
	path, err := CreateNote(a.vault)
	if err != nil {
		a.fail("error.create", err)
		return
	}
	a.selected, a.text, a.saved, a.query = path, "", "", ""
	if a.mode == modePreview {
		a.mode = modeEdit
	}
	a.focus = editorKey(a.selected)
	a.reload()
}

// mode is how the open note shows: rendered, raw beside rendered, or raw.
type mode string

const (
	modePreview mode = "preview"
	modeSplit   mode = "split"
	modeEdit    mode = "edit"
	// defaultSplit is the editor's share when a split first opens.
	defaultSplit = 0.5
)

func (a *app) setMode(m mode) {
	switch m {
	case modeSplit, modeEdit:
		a.focus = editorKey(a.selected)
	case modePreview:
	default:
		return
	}
	a.mode = m
}

func (m mode) editing() bool { return m == modeSplit || m == modeEdit }

func editorKey(path string) string { return "editor:" + path }

// save writes the open note when it has changes and reports whether nothing
// is left unsaved.
func (a *app) save() bool {
	if a.selected == "" || a.text == a.saved {
		return true
	}
	if err := SaveNote(a.vault, a.selected, a.text); err != nil {
		a.fail("error.save", err)
		return false
	}
	a.saved = a.text
	a.reload()
	a.send(extension.ShowToast.New(extension.ToastData{
		Variant: extension.ToastSuccess, Title: a.t("saved", i18n.Args{"title": a.title()}),
	}))
	return true
}

func (a *app) title() string {
	for _, n := range a.notes {
		if n.Path == a.selected {
			return n.Title
		}
	}
	return a.selected
}

func (a *app) fail(key string, err error) {
	log.Printf("%s: %v", key, err)
	a.send(extension.ShowToast.New(extension.ToastData{
		Variant: extension.ToastError, Title: a.t(key), Description: err.Error(),
	}))
}
