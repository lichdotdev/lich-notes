# LichNotes

A notes screen for [lich](https://github.com/lichdotdev/lich), and the reference example of a lich extension.
Read it to see how an extension is put together, then copy what you need into your own.

![The notes screen: a list of notes on the left, the open note rendered on the right](docs/screenshot.png)

## What it does

- Lists the Markdown notes of a folder you choose (a vault), newest first. Sub-folders are included and
  folders whose name starts with a dot are skipped.
- Filters the list as you type, by title and by content.
- Shows the selected note three ways: **Preview** renders it, **Split** puts the Markdown source beside the
  rendered note and updates it as you type, **Edit** shows the source alone. **Save** writes the file, and
  saving from Edit goes back to the preview.
- Creates a new note as `Untitled N.md`.
- Deletes the selected note after confirmation; unsaved changes are deleted with it.

| Key | Does |
| --- | --- |
| `/` | Puts the keyboard in the search field |
| `Ctrl+N` | New note |
| `Ctrl+S` | Save, in Split or Edit |
| `Esc` | Hands the keyboard back, so `/` works again |

A note you changed is saved when you open another one, so nothing typed is lost by clicking away.
Deleting a note removes its Markdown file from the vault permanently.

The first time you open the screen it asks for the vault's path. The folder button beside the search field
changes it later. The interface follows lich's language, in English and Brazilian Portuguese.

## Install

lich lists extensions in **Settings › Extensions**. Open the **Notes** screen from the button it adds to the
top bar.

### From a release

Choose **From GitHub**, paste `https://github.com/lichdotdev/lich-notes` and confirm. lich downloads the
archive of the latest release built for your system and checks it against the release's checksums.

### From a folder

Build it, and put the three things lich reads in a folder of their own:

```sh
go build -o lich-notes .
mkdir -p /tmp/lich-notes
cp -R manifest.json lich-notes locales /tmp/lich-notes/
```

Choose **From a folder** and pick `/tmp/lich-notes`. lich copies the folder, so you can delete it afterwards.
On Windows build `lich-notes.exe` and name it in the manifest's `executable`.

## Develop

lich reads a linked folder too, which is how you run the code you are editing. Link this checkout into the
extensions directory under the extension's id:

```sh
go build -o lich-notes .
ln -s "$PWD" ~/.config/lich/extensions/lich-notes
```

The directory is `~/Library/Application Support/lich/extensions` on macOS and `%AppData%\lich\extensions` on
Windows. A lich run with `LICH_DEV` set reads `extensions-dev` instead. Restart lich to pick the link up.

After a change, build again and restart lich. What the extension prints to stderr goes to lich's log.

```sh
go vet ./... && go test ./...
```

## How the code is laid out

An extension is a program lich starts and talks to over JSON lines on stdin and stdout. It does not draw: it
sends a tree of components and lich draws them. The Go package
[`github.com/lichdotdev/lich/extension`](https://github.com/lichdotdev/lich/tree/main/extension) has the
types, so no JSON is written by hand here.

| File | Holds |
| --- | --- |
| `manifest.json` | What lich reads before starting the program: its id, the executable, the screen it adds |
| `locales/<tag>/` | The translations. `extension.json` names the extension and its screen, `notes.json` is the screen's own text |
| `main.go` | Start-up: load the catalogs and the settings, then answer lich's messages until it closes stdin |
| `app.go` | The state of the screen and what each message from lich changes in it |
| `view.go` | The component tree built from that state, and the keyboard shortcuts |
| `vault.go` | The notes on disk: list, search, read, save, create, delete. Knows nothing about lich |
| `settings.go` | The vault's path, kept in the data directory lich gives the extension |
| `release.sh` | Builds the archives and `checksums.txt` a release carries |

The loop in three steps, which every extension repeats:

1. lich sends a message: the screen opened, a control fired the handler the tree named for it, or the
   language changed.
2. `app.handle` changes the state.
3. `app.render` sends the whole tree again. lich keeps what the person typed and where they scrolled, as
   long as a node keeps its key.

Things here that are easy to get wrong in your own:

- **Save into the data directory**, not beside the executable: an update replaces the extension's folder.
  lich passes the path in `LICH_EXTENSION_DATA`.
- **Send `focus` after the render** that draws the field. lich drops a focus for a field it has not drawn.
- **Key a field by what it edits.** The editor here is keyed by the note's path, so one note's caret never
  lands in another.
- **Check paths that come back in events.** A row's handler carries the note's path, and `vault.go` refuses
  one that leaves the vault.

The full contract is in
[Writing a lich extension](https://github.com/lichdotdev/lich/blob/main/docs/extensions.md).

## Release

Bump `version` in `manifest.json`, commit, then push a tag of the same version:

```sh
git tag v0.1.0 && git push origin v0.1.0
```

`.github/workflows/release.yml` runs the tests and `release.sh`, and publishes
`lich-notes_<os>_<arch>.tar.gz` for Linux, macOS and Windows on amd64 and arm64 with `checksums.txt`, the
shape lich installs from. `./release.sh v0.1.0` builds the same files into `dist/` on your machine.

## License

[AGPL-3.0](LICENSE), the same as lich.
