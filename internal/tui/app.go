package tui

import (
	"context"
	"time"

	"io"

	"github.com/gdamore/tcell/v2"
	"github.com/google/uuid"
	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"github.com/matheus3301/wpp/internal/tui/client"
	"github.com/matheus3301/wpp/internal/tui/keys"
	"github.com/matheus3301/wpp/internal/tui/model"
	"github.com/matheus3301/wpp/internal/tui/views"
	"github.com/rivo/tview"
)

// App is the main TUI application shell.
type App struct {
	app       *tview.Application
	pages     *tview.Pages
	vm        *model.ViewModel
	grpc      *client.Client
	registry  *keys.Registry
	statusBar *views.StatusBar
	chatList  *views.ChatList
	msgView   *views.MessageView
	composer  *views.Composer
	searchV   *views.SearchView
	authView  *views.AuthView
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewApp creates the TUI application.
func NewApp(c *client.Client, sessionName string) *App {
	ctx, cancel := context.WithCancel(context.Background())
	vm := model.NewViewModel(c)

	a := &App{
		app:       tview.NewApplication(),
		pages:     tview.NewPages(),
		vm:        vm,
		grpc:      c,
		registry:  keys.NewRegistry(),
		statusBar: views.NewStatusBar(),
		chatList:  views.NewChatList(),
		msgView:   views.NewMessageView(),
		composer:  views.NewComposer(),
		searchV:   views.NewSearchView(),
		authView:  views.NewAuthView(),
		ctx:       ctx,
		cancel:    cancel,
	}

	a.statusBar.SetSession(sessionName)
	a.setupBindings()
	a.setupCallbacks()
	a.setupLayout()

	return a
}

func (a *App) setupBindings() {
	a.registry.AddGlobal("quit", &keys.Action{
		Rune: 'q', Key: tcell.KeyRune,
		Description: "q:quit", Visible: true,
		Handler: func() { a.app.Stop() },
	})
	a.registry.AddGlobal("search", &keys.Action{
		Rune: 's', Key: tcell.KeyRune,
		Description: "s:search", Visible: true,
		Handler: func() { a.showSearch() },
	})
	a.registry.AddGlobal("help", &keys.Action{
		Rune: '?', Key: tcell.KeyRune,
		Description: "?:help", Visible: true,
		Handler: func() {},
	})
}

func (a *App) setupCallbacks() {
	a.chatList.SetSelectionChangedFunc(func(row, col int) {
		// Selection changed, no-op.
	})

	a.chatList.SetSelectedFunc(func(row, col int) {
		jid := a.chatList.SelectedChat()
		if jid != "" {
			a.openChat(jid)
		}
	})

	a.composer.SetOnSend(func(text string) {
		chatJID := a.vm.ActiveChatJID
		if chatJID == "" {
			return
		}
		go func() {
			clientMsgID := uuid.New().String()
			if err := a.vm.SendText(a.ctx, chatJID, text, clientMsgID); err != nil {
				a.vm.Flash.Set("Send failed: "+err.Error(), 5*time.Second)
			}
			_ = a.vm.LoadMessages(a.ctx, chatJID)
			a.app.QueueUpdateDraw(func() {
				a.msgView.Update(a.vm.GetMessages())
				a.statusBar.SetFlash(a.vm.Flash.Get())
			})
		}()
	})

	a.searchV.SetOnQuery(func(query string) {
		go func() {
			results, err := a.vm.SearchMessages(a.ctx, query)
			if err != nil {
				a.vm.Flash.Set("Search failed: "+err.Error(), 5*time.Second)
				return
			}
			a.app.QueueUpdateDraw(func() {
				a.searchV.Update(results)
				a.app.SetFocus(a.searchV.Results())
			})
		}()
	})
}

func (a *App) setupLayout() {
	chatFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.msgView, 0, 1, false).
		AddItem(a.composer, 1, 0, false)

	a.pages.AddPage("chats", a.chatList, true, true)
	a.pages.AddPage("chat", chatFlex, true, false)
	a.pages.AddPage("search", a.searchV, true, false)
	a.pages.AddPage("auth", a.authView, true, false)

	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.statusBar, 1, 0, false)

	a.app.SetRoot(root, true)

	a.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		currentPage, _ := a.pages.GetFrontPage()

		if event.Key() == tcell.KeyEscape {
			switch currentPage {
			case "chat", "search", "auth":
				a.pages.SwitchToPage("chats")
				a.app.SetFocus(a.chatList)
				return nil
			}
		}

		// Let text input widgets handle all keys normally.
		focused := a.app.GetFocus()
		if _, ok := focused.(*tview.InputField); ok {
			return event
		}

		// 'i' focuses the composer (only when not already in an input field).
		if currentPage == "chat" && event.Key() == tcell.KeyRune && event.Rune() == 'i' {
			a.app.SetFocus(a.composer.InputField)
			return nil
		}

		if a.registry.HandleEvent(currentPage, event) {
			return nil
		}

		return event
	})
}

func (a *App) openChat(jid string) {
	go func() {
		if err := a.vm.LoadMessages(a.ctx, jid); err != nil {
			a.vm.Flash.Set("Load failed: "+err.Error(), 5*time.Second)
			return
		}
		chatName := jid
		for _, c := range a.vm.GetChats() {
			if c.Jid == jid {
				if c.Name != "" {
					chatName = c.Name
				}
				break
			}
		}
		a.app.QueueUpdateDraw(func() {
			a.msgView.SetChatName(chatName)
			a.msgView.Update(a.vm.GetMessages())
			a.pages.SwitchToPage("chat")
			a.app.SetFocus(a.msgView)
		})
	}()
}

func (a *App) showSearch() {
	a.pages.SwitchToPage("search")
	a.app.SetFocus(a.searchV.Input())
}

// Run starts the TUI application.
func (a *App) Run() error {
	go func() {
		_ = a.vm.LoadSessionStatus(a.ctx)
		_ = a.vm.LoadSyncStatus(a.ctx)
		_ = a.vm.LoadChats(a.ctx)

		a.app.QueueUpdateDraw(func() {
			a.chatList.Update(a.vm.GetChats())

			ss := a.vm.GetSessionStatus()
			if ss != nil {
				a.statusBar.SetStatus(ss.StatusMessage)
				if ss.Status == wppv1.SessionStatus_SESSION_STATUS_AUTH_REQUIRED {
					a.pages.SwitchToPage("auth")
					a.authView.ShowMessage("Starting authentication...")
					go a.runAuthFlow()
				}
			}
		})

		a.startRefreshLoop()
	}()

	return a.app.Run()
}

func (a *App) startRefreshLoop() {
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				_ = a.vm.LoadChats(a.ctx)
				_ = a.vm.LoadSessionStatus(a.ctx)
				a.app.QueueUpdateDraw(func() {
					currentPage, _ := a.pages.GetFrontPage()
					if currentPage == "chats" {
						a.chatList.Update(a.vm.GetChats())
					}
					ss := a.vm.GetSessionStatus()
					if ss != nil {
						a.statusBar.SetStatus(ss.StatusMessage)
						if currentPage == "auth" && ss.Status != wppv1.SessionStatus_SESSION_STATUS_AUTH_REQUIRED {
							a.chatList.Update(a.vm.GetChats())
							a.pages.SwitchToPage("chats")
							a.app.SetFocus(a.chatList)
						}
					}
					a.statusBar.SetFlash(a.vm.Flash.Get())
				})
			case <-a.ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

// runAuthFlow calls StartAuth on the daemon and streams QR codes to the auth view.
func (a *App) runAuthFlow() {
	stream, err := a.grpc.Session.StartAuth(a.ctx, &wppv1.StartAuthRequest{})
	if err != nil {
		a.app.QueueUpdateDraw(func() {
			a.authView.ShowMessage("Auth error: " + err.Error())
		})
		return
	}

	for {
		evt, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			a.app.QueueUpdateDraw(func() {
				a.authView.ShowMessage("Auth stream error: " + err.Error())
			})
			return
		}

		switch evt.EventType {
		case "qr_code":
			a.app.QueueUpdateDraw(func() {
				a.authView.ShowQR(evt.QrCode)
			})
		case "authenticated":
			a.app.QueueUpdateDraw(func() {
				a.authView.ShowMessage("Authenticated! Loading chats...")
				a.statusBar.SetStatus("READY")
				// Reload data and switch to chat list.
				go func() {
					_ = a.vm.LoadSessionStatus(a.ctx)
					_ = a.vm.LoadChats(a.ctx)
					a.app.QueueUpdateDraw(func() {
						a.chatList.Update(a.vm.GetChats())
						a.pages.SwitchToPage("chats")
						a.app.SetFocus(a.chatList)
					})
				}()
			})
			return
		case "auth_failed", "timeout":
			msg := evt.Message
			if msg == "" {
				msg = "Authentication failed"
			}
			a.app.QueueUpdateDraw(func() {
				a.authView.ShowMessage(msg)
			})
			return
		}
	}
}

// Stop gracefully shuts down the TUI.
func (a *App) Stop() {
	a.cancel()
	a.app.Stop()
}
