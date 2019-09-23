package update

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/fluxcd/flux/pkg/resource"
)

const (
	// Escape sequences.
	moveCursorUp = "\033[%dA"
	hideCursor   = "\033[?25l"
	showCursor   = "\033[?25h"

	// Glyphs.
	glyphSelected  = "\u21d2"
	glyphChecked   = "\u25c9"
	glyphUnchecked = "\u25ef"

	tableHeading = "WORKLOAD \tSTATUS \tUPDATES"
)

type writer struct {
	out   io.Writer
	tw    *tabwriter.Writer
	lines int    // lines written since last clear
	width uint16 // terminal width
}

func newWriter(out io.Writer) *writer {
	return &writer{
		out:   out,
		tw:    tabwriter.NewWriter(out, 0, 2, 2, ' ', 0),
		width: terminalWidth(),
	}
}

func (c *writer) hideCursor() {
	fmt.Fprintf(c.out, hideCursor)
}

func (c *writer) showCursor() {
	fmt.Fprintf(c.out, showCursor)
}

// writeln counts the lines we output.
func (c *writer) writeln(line string) error {
	line += "\n"
	c.lines += (len(line)-1)/int(c.width) + 1
	_, err := c.tw.Write([]byte(line))
	return err
}

// clear moves the terminal cursor up to the beginning of the
// line where we started writing.
func (c *writer) clear() {
	if c.lines != 0 {
		fmt.Fprintf(c.out, moveCursorUp, c.lines)
	}
	c.lines = 0
}

func (c *writer) flush() error {
	return c.tw.Flush()
}

type menuItem struct {
	id     resource.ID
	status WorkloadUpdateStatus
	error  string
	update ContainerUpdate

	checked bool
}

// Menu presents a list of controllers which can be interacted with.
type Menu struct {
	wr         *writer
	items      []menuItem
	selectable int
	cursor     int
}

// NewMenu creates a menu printer that outputs a result set to
// the `io.Writer` provided, at the given level of verbosity:
//  - 2 = include skipped and ignored resources
//  - 1 = include skipped resources, exclude ignored resources
//  - 0 = exclude skipped and ignored resources
//
// It can print a one time listing with `Print()` or then enter
// interactive mode with `Run()`.
func NewMenu(out io.Writer, results Result, verbosity int) *Menu {
	m := &Menu{wr: newWriter(out)}
	m.fromResults(results, verbosity)
	return m
}

func (m *Menu) fromResults(results Result, verbosity int) {
	for _, workloadID := range results.WorkloadIDs() {
		resourceID := resource.MustParseID(workloadID)
		result := results[resourceID]
		switch result.Status {
		case ReleaseStatusIgnored:
			if verbosity < 2 {
				continue
			}
		case ReleaseStatusSkipped:
			if verbosity < 1 {
				continue
			}
		}

		if result.Error != "" {
			m.AddItem(menuItem{
				id:     resourceID,
				status: result.Status,
				error:  result.Error,
			})
		}
		for _, upd := range result.PerContainer {
			m.AddItem(menuItem{
				id:     resourceID,
				status: result.Status,
				update: upd,
			})
		}
		if result.Error == "" && len(result.PerContainer) == 0 {
			m.AddItem(menuItem{
				id:     resourceID,
				status: result.Status,
			})
		}
	}
	return
}

func (m *Menu) AddItem(mi menuItem) {
	if mi.checkable() {
		mi.checked = true
		m.selectable++
	}
	m.items = append(m.items, mi)
}

// Run starts the interactive menu mode.
func (m *Menu) Run() (map[resource.ID][]ContainerUpdate, error) {
	specs := make(map[resource.ID][]ContainerUpdate)
	if m.selectable == 0 {
		return specs, errors.New("No changes found.")
	}

	m.printInteractive()
	m.wr.hideCursor()
	defer m.wr.showCursor()

	for {
		ascii, keyCode, err := getChar()
		if err != nil {
			return specs, err
		}

		switch ascii {
		case 3, 27, 'q':
			return specs, errors.New("Aborted.")
		case ' ':
			m.toggleSelected()
		case 13:
			for _, item := range m.items {
				if item.checked {
					specs[item.id] = append(specs[item.id], item.update)
				}
			}
			m.wr.writeln("")
			return specs, nil
		case 9, 'j':
			m.cursorDown()
		case 'k':
			m.cursorUp()
		default:
			switch keyCode {
			case 40:
				m.cursorDown()
			case 38:
				m.cursorUp()
			}
		}

	}
}

func (m *Menu) Print() {
	m.wr.writeln(tableHeading)
	var previd resource.ID
	for _, item := range m.items {
		inline := previd == item.id
		m.wr.writeln(m.renderItem(item, inline))
		previd = item.id
	}
	m.wr.flush()
}

func (m *Menu) printInteractive() {
	m.wr.clear()
	m.wr.writeln("    " + tableHeading)
	i := 0
	var previd resource.ID
	for _, item := range m.items {
		inline := previd == item.id
		m.wr.writeln(m.renderInteractiveItem(item, inline, i))
		previd = item.id
		if item.checkable() {
			i++
		}
	}
	m.wr.writeln("")
	m.wr.writeln("Use arrow keys and [Space] to toggle updates; hit [Enter] to release selected.")

	m.wr.flush()
}

func (m *Menu) renderItem(item menuItem, inline bool) string {
	if inline {
		return fmt.Sprintf("\t\t%s", item.updates())
	} else {
		return fmt.Sprintf("%s\t%s\t%s", item.id, item.status, item.updates())
	}
}

func (m *Menu) renderInteractiveItem(item menuItem, inline bool, index int) string {
	pre := bytes.Buffer{}
	if index == m.cursor {
		pre.WriteString(glyphSelected)
	} else {
		pre.WriteString(" ")
	}
	pre.WriteString(" ")
	pre.WriteString(item.checkbox())
	pre.WriteString(" ")
	pre.WriteString(m.renderItem(item, inline))

	return pre.String()
}

func (m *Menu) toggleSelected() {
	m.items[m.cursor].checked = !m.items[m.cursor].checked
	m.printInteractive()
}

func (m *Menu) cursorDown() {
	m.cursor = (m.cursor + 1) % m.selectable
	m.printInteractive()
}

func (m *Menu) cursorUp() {
	m.cursor = (m.cursor + m.selectable - 1) % m.selectable
	m.printInteractive()
}

func (i menuItem) checkbox() string {
	switch {
	case !i.checkable():
		return " "
	case i.checked:
		return glyphChecked
	default:
		return glyphUnchecked
	}
}

func (i menuItem) checkable() bool {
	return i.update.Container != ""
}

func (i menuItem) updates() string {
	if i.update.Container != "" {
		return fmt.Sprintf("%s: %s -> %s",
			i.update.Container,
			i.update.Current.String(),
			i.update.Target.Tag)
	}
	return i.error
}
