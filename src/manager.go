package prockeeper

import (
	"flag"
	"log"

	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

// Manager ...
type Manager struct {
	Services []*Service
	list     *tview.List
	logger   *log.Logger
}

var debug bool

func init() {
	flag.BoolVar(&debug, "debug", false, "debug mode")
	flag.Parse()
}

func (manager *Manager) refreshList() {
	currentSelection := manager.list.GetCurrentItem()
	manager.list.Clear()
	for _, s := range manager.Services {
		manager.list.AddItem(s.NameWithPid(), "", 0, nil)
	}
	manager.list.SetCurrentItem(currentSelection)
}

func (manager *Manager) startAll() {
	for _, s := range manager.Services {
		go func(s *Service) {
			if err := s.Start(); err != nil {
				manager.logger.Println(err)
			}
		}(s)
	}
}

func (manager *Manager) stopAll() {
	for _, s := range manager.Services {
		go func(s *Service) {
			if err := s.Stop(); err != nil {
				manager.logger.Println(err)
			}
		}(s)
	}
}

// Run ...
func (manager *Manager) Run() {
	config := ParseConfig()

	app := tview.NewApplication()

	list := tview.NewList().ShowSecondaryText(false)
	manager.list = list
	list.SetTitle("Services (Press ? to show help)").SetBorder(true)

	debugger := tview.NewTextView().
		SetDynamicColors(true).
		SetChangedFunc(func() {
			app.Draw()
		})
	debugger.SetTitle("debugger").SetBorder(true)

	serviceLog := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetChangedFunc(func() {
			app.Draw()
		})
	serviceLog.SetBorder(true)

	help := HelpMenu()

	layout := tview.NewFlex().SetDirection(tview.FlexRow)
	pages := tview.NewPages().
		AddPage("app", layout, true, true).
		AddPage("help", modal(help, 40, 20), true, false)
	app.SetRoot(pages, true)

	appContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	appContainer.AddItem(list, 0, 2, true)
	appContainer.AddItem(serviceLog, 0, 6, true)
	layout.AddItem(appContainer, 0, 5, false)

	debuggerContainer := tview.NewFlex()
	debuggerContainer.AddItem(debugger, 0, 1, true)
	if debug {
		layout.AddItem(debuggerContainer, 0, 1, false)
	}


	logger := log.New(debugger, "", log.LstdFlags)
	manager.logger = logger

	updated := make(chan struct{})
	go func() {
		for range updated {
			logger.Println("refresh list")
			manager.refreshList()
		}
	}()

	for _, s := range config.Services {
		service := NewService(s.Name, s.Command, updated, logger, serviceLog)
		manager.Services = append(manager.Services, service)
	}
	currentService := manager.Services[0]
	manager.refreshList()

	list.SetChangedFunc(func(i int, n string, v string, t rune) {
		currentService.PauseStdout()
		s := manager.Services[i]
		serviceLog.SetTitle(s.Command)
		serviceLog.SetText(s.History.String())
		s.ResumeStdout()
		currentService = s
	})

	list.SetSelectedFunc(func(i int, n string, v string, t rune) {
		manager.Services[i].Toggle()
	})

	exitMenu := tview.NewModal().
		SetText("Running services!").
		AddButtons([]string{"Force Quit", "Cancel"}).
		SetFocus(1).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Force Quit" {
				app.Stop()
			} else {
				pages.RemovePage("exit")
				app.SetFocus(list)
			}
		})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == '?' {
			if name, _ := pages.GetFrontPage(); name == "help" {
				pages.HidePage("help")
				app.SetFocus(list)
			} else {
				pages.ShowPage("help")
			}
			return nil
		}
		if event.Rune() == '.' {
			if debug {
				layout.RemoveItem(debuggerContainer)
				debug = false
			} else {
				layout.AddItem(debuggerContainer, 0, 1, false)
				debug = true
			}
		}
		if event.Rune() == 'j' {
			return tcell.NewEventKey(tcell.KeyDown, 'j', tcell.ModNone)
		}
		if event.Rune() == 'k' {
			return tcell.NewEventKey(tcell.KeyUp, 'k', tcell.ModNone)
		}
		if event.Key() == tcell.KeyCtrlC {
			allStopped := true
			for _, s := range manager.Services {
				if s.Cmd != nil {
					allStopped = false
					break
				}
			}
			if allStopped {
				return event
			}
			pages.AddPage("exit", exitMenu, true, true)
			return nil
		}
		if event.Rune() == 'u' {
			manager.startAll()
			return nil
		}
		if event.Rune() == 'd' {
			manager.stopAll()
			return nil
		}
		return event
	})

	if err := app.SetFocus(list).Run(); err != nil {
		panic(err)
	}
}