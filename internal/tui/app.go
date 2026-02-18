package tui

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/google/uuid"
	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"github.com/matheus3301/wpp/internal/tui/client"
	"github.com/matheus3301/wpp/internal/tui/model"
	"github.com/matheus3301/wpp/internal/tui/ui"
	"github.com/matheus3301/wpp/internal/tui/views"
	"github.com/rivo/tview"
)

// App is the main TUI application shell.
type App struct {
	app     *tview.Application
	theme   *ui.Theme
	pages   *ui.Pages
	vm      *model.ViewModel
	grpc    *client.Client
	session string

	// Header components.
	sessionInfo *ui.SessionInfo
	menu        *ui.Menu
	logo        *ui.Logo

	// Layout components.
	prompt   *ui.Prompt
	crumbs   *ui.Crumbs
	flashBar *ui.FlashBar
	header   *tview.Flex
	root     *tview.Flex

	// Prompt visibility state.
	promptVisible bool
	promptRow     *tview.Flex

	// Views.
	convList  *views.ConversationList
	msgThread *views.MessageThread
	convInfo  *views.ConversationInfo
	searchV   *views.SearchView
	authView  *views.AuthView
	helpView  *views.HelpView

	ctx    context.Context
	cancel context.CancelFunc
}

// NewApp creates the TUI application.
func NewApp(c *client.Client, sessionName string) *App {
	ctx, cancel := context.WithCancel(context.Background())
	theme := ui.DefaultTheme()
	vm := model.NewViewModel(c)

	a := &App{
		app:         tview.NewApplication(),
		theme:       theme,
		pages:       ui.NewPages(),
		vm:          vm,
		grpc:        c,
		session:     sessionName,
		sessionInfo: ui.NewSessionInfo(theme),
		menu:        ui.NewMenu(theme),
		logo:        ui.NewLogo(theme),
		prompt:      ui.NewPrompt(theme),
		crumbs:      ui.NewCrumbs(theme),
		flashBar:    ui.NewFlashBar(theme),
		convList:    views.NewConversationList(theme),
		msgThread:   views.NewMessageThread(theme),
		convInfo:    views.NewConversationInfo(theme),
		searchV:     views.NewSearchView(theme),
		authView:    views.NewAuthView(theme),
		helpView:    views.NewHelpView(theme),
		ctx:         ctx,
		cancel:      cancel,
	}

	a.setupCallbacks()
	a.setupLayout()
	a.setupInputCapture()

	return a
}

func (a *App) setupCallbacks() {
	// Conversation list: open selected chat.
	a.convList.SetSelectedFunc(func(row, col int) {
		jid := a.convList.SelectedChat()
		if jid != "" {
			a.openChat(jid)
		}
	})

	// Message thread: send message.
	a.msgThread.SetOnSend(func(text string) {
		chatJID := a.vm.ActiveChatJID
		if chatJID == "" {
			return
		}
		go func() {
			clientMsgID := uuid.New().String()
			if err := a.vm.SendText(a.ctx, chatJID, text, clientMsgID); err != nil {
				a.vm.FlashUI.Err(err)
				a.vm.SignalRefresh()
			}
		}()
	})

	// Search view: query.
	a.searchV.SetOnQuery(func(query string) {
		go func() {
			results, err := a.vm.SearchMessages(a.ctx, query)
			if err != nil {
				a.vm.FlashUI.Err(err)
				return
			}
			a.app.QueueUpdateDraw(func() {
				a.searchV.Update(results)
				a.app.SetFocus(a.searchV.Results())
			})
		}()
	})

	// Prompt: submit and cancel.
	a.prompt.SetOnSubmit(func(mode ui.PromptMode, text string) {
		a.hidePrompt()
		switch mode {
		case ui.PromptCommand:
			a.executeCommand(text)
		case ui.PromptFilter:
			a.applyFilter(text)
		}
	})

	a.prompt.SetOnCancel(func() {
		a.hidePrompt()
		// Clear filter when cancelling filter mode.
		if a.pages.Current() == "conversations" {
			a.convList.ClearFilter()
		}
	})

	// Pages: update crumbs and menu on stack changes.
	a.pages.SetOnChange(func(stack []string) {
		a.crumbs.Update(stack)
		a.updateMenu()
	})
}

func (a *App) setupLayout() {
	// Register pages.
	a.pages.AddPage("conversations", a.convList, true, false)
	a.pages.AddPage("messages", a.msgThread, true, false)
	a.pages.AddPage("details", a.convInfo, true, false)
	a.pages.AddPage("search", a.searchV, true, false)
	a.pages.AddPage("auth", a.authView, true, false)
	a.pages.AddPage("help", a.helpView, true, false)

	// Header: SessionInfo (fixed) | Menu (flex) | Logo (fixed).
	a.header = tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(a.sessionInfo, 50, 0, false).
		AddItem(a.menu, 0, 1, false).
		AddItem(a.logo, 16, 0, false)
	a.header.SetBackgroundColor(a.theme.BgColor)

	// Prompt row (hidden by default).
	a.promptRow = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.prompt, 3, 0, false)

	// Root layout.
	a.root = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.header, 7, 0, false).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.crumbs, 1, 0, false).
		AddItem(a.flashBar, 1, 0, false)

	a.app.SetRoot(a.root, true)

	// Push initial page.
	a.pages.Push("conversations")
	a.updateMenu()
}

func (a *App) setupInputCapture() {
	a.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		focused := a.app.GetFocus()

		// Esc in composer: unfocus composer, return to messages view.
		if event.Key() == tcell.KeyEscape {
			if inp, ok := focused.(*tview.InputField); ok && inp == a.msgThread.Composer() {
				a.app.SetFocus(a.msgThread.Messages())
				return nil
			}
		}

		// Let other text input widgets (prompt, search) handle all keys.
		if _, ok := focused.(*tview.InputField); ok {
			return event
		}

		currentPage := a.pages.Current()

		// Ctrl-C: quit immediately.
		if event.Key() == tcell.KeyCtrlC {
			a.app.Stop()
			return nil
		}

		// Escape: hide prompt, clear filter, or pop stack.
		if event.Key() == tcell.KeyEscape {
			if a.promptVisible {
				a.hidePrompt()
				if currentPage == "conversations" {
					a.convList.ClearFilter()
				}
				return nil
			}
			// Clear active filter on conversations page.
			if currentPage == "conversations" {
				a.convList.ClearFilter()
				return nil
			}
			if a.pages.Depth() > 1 {
				a.pages.Pop()
				a.focusCurrentPage()
				return nil
			}
			return nil
		}

		if event.Key() != tcell.KeyRune {
			return event
		}

		r := event.Rune()

		// ':' command mode.
		if r == ':' {
			a.showPrompt(ui.PromptCommand)
			return nil
		}

		// '/' filter mode.
		if r == '/' {
			a.showPrompt(ui.PromptFilter)
			return nil
		}

		// '?' help.
		if r == '?' {
			a.pushView("help")
			return nil
		}

		// 'q' pop or quit.
		if r == 'q' {
			if a.pages.Depth() > 1 {
				a.pages.Pop()
				a.focusCurrentPage()
			} else {
				a.app.Stop()
			}
			return nil
		}

		// Conversation list specific keys.
		if currentPage == "conversations" {
			// Numeric shortcuts 0-9.
			if r == '0' {
				a.convList.ClearFilter()
				return nil
			}
			if r >= '1' && r <= '9' {
				n := int(r - '0')
				jid := a.convList.ChatByIndex(n)
				if jid != "" {
					a.openChat(jid)
				}
				return nil
			}
			if r == 's' {
				// Sort cycling (future enhancement).
				return nil
			}
		}

		// Message thread specific keys.
		if currentPage == "messages" {
			if r == 'i' {
				a.app.SetFocus(a.msgThread.Composer())
				return nil
			}
			if r == 'd' {
				chat := a.vm.GetChatByJID(a.msgThread.ChatJID())
				if chat != nil {
					a.convInfo.Update(chat)
				}
				a.pushView("details")
				return nil
			}
		}

		return event
	})
}

func (a *App) showPrompt(mode ui.PromptMode) {
	if a.promptVisible {
		return
	}
	a.promptVisible = true
	a.prompt.Activate(mode)

	// Insert prompt row before the content area.
	a.root.Clear()
	a.root.
		AddItem(a.header, 7, 0, false).
		AddItem(a.promptRow, 3, 0, false).
		AddItem(a.pages, 0, 1, false).
		AddItem(a.crumbs, 1, 0, false).
		AddItem(a.flashBar, 1, 0, false)

	a.app.SetFocus(a.prompt)
}

func (a *App) hidePrompt() {
	if !a.promptVisible {
		return
	}
	a.promptVisible = false

	a.root.Clear()
	a.root.
		AddItem(a.header, 7, 0, false).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.crumbs, 1, 0, false).
		AddItem(a.flashBar, 1, 0, false)

	a.focusCurrentPage()
}

func (a *App) executeCommand(text string) {
	cmd := ParseCommand(text)
	switch cmd.Name {
	case "search", "s":
		if cmd.Args != "" {
			a.pushView("search")
			go func() {
				results, err := a.vm.SearchMessages(a.ctx, cmd.Args)
				if err != nil {
					a.vm.FlashUI.Err(err)
					return
				}
				a.app.QueueUpdateDraw(func() {
					a.searchV.Update(results)
					a.app.SetFocus(a.searchV.Results())
				})
			}()
		} else {
			a.pushView("search")
			a.app.SetFocus(a.searchV.Input())
		}
	case "chat", "c":
		if cmd.Args != "" {
			a.openChatByName(cmd.Args)
		}
	case "logout":
		go func() {
			_, err := a.grpc.Session.Logout(a.ctx, &wppv1.LogoutRequest{})
			if err != nil {
				a.vm.FlashUI.Err(err)
			} else {
				a.vm.FlashUI.Info("Logged out")
			}
			a.vm.SignalRefresh()
		}()
	case "help", "h":
		a.pushView("help")
	case "quit", "q":
		a.app.Stop()
	default:
		a.vm.FlashUI.Warn("Unknown command: " + cmd.Name)
	}
}

func (a *App) applyFilter(text string) {
	if a.pages.Current() == "conversations" {
		a.convList.SetFilter(text)
	}
}

func (a *App) openChat(jid string) {
	go func() {
		if err := a.vm.LoadMessages(a.ctx, jid); err != nil {
			a.vm.FlashUI.Err(err)
			return
		}
		chatName := jid
		chat := a.vm.GetChatByJID(jid)
		if chat != nil && chat.Name != "" {
			chatName = chat.Name
		}
		a.app.QueueUpdateDraw(func() {
			a.msgThread.SetChatName(chatName)
			a.msgThread.SetChatJID(jid)
			a.msgThread.Update(a.vm.GetMessages())
			a.pushView("messages")
		})
	}()
}

func (a *App) openChatByName(name string) {
	name = strings.ToLower(name)
	for _, chat := range a.vm.GetChats() {
		chatName := chat.Name
		if chatName == "" {
			chatName = chat.Jid
		}
		if strings.Contains(strings.ToLower(chatName), name) {
			a.openChat(chat.Jid)
			return
		}
	}
	a.vm.FlashUI.Warn("No chat matching: " + name)
}

func (a *App) pushView(name string) {
	a.pages.Push(name)
	a.focusCurrentPage()
}

func (a *App) focusCurrentPage() {
	switch a.pages.Current() {
	case "conversations":
		a.app.SetFocus(a.convList)
	case "messages":
		a.app.SetFocus(a.msgThread.Messages())
	case "search":
		a.app.SetFocus(a.searchV.Input())
	case "auth":
		a.app.SetFocus(a.authView)
	case "help":
		a.app.SetFocus(a.helpView)
	case "details":
		a.app.SetFocus(a.convInfo)
	}
}

func (a *App) updateMenu() {
	var hints []ui.MenuHint
	switch a.pages.Current() {
	case "conversations":
		hints = a.convList.Hints()
	case "messages":
		hints = a.msgThread.Hints()
	case "search":
		hints = a.searchV.Hints()
	case "auth":
		hints = a.authView.Hints()
	case "help":
		hints = a.helpView.Hints()
	case "details":
		hints = a.convInfo.Hints()
	}
	a.menu.Update(hints)
}

// Run starts the TUI application.
func (a *App) Run() error {
	go func() {
		_ = a.vm.LoadSessionStatus(a.ctx)
		_ = a.vm.LoadSyncStatus(a.ctx)
		_ = a.vm.LoadChats(a.ctx)

		a.app.QueueUpdateDraw(func() {
			a.convList.Update(a.vm.GetChats())
			a.sessionInfo.Update(a.vm.GetSessionInfo())

			ss := a.vm.GetSessionStatus()
			if ss != nil {
				if ss.Status == wppv1.SessionStatus_SESSION_STATUS_AUTH_REQUIRED {
					a.pushView("auth")
					a.authView.ShowMessage("Starting authentication...")
					go a.runAuthFlow()
				}
			}
		})

		a.vm.StartWatchingMessages(a.ctx)
		a.vm.StartWatchingChats(a.ctx)
		a.startRefreshLoop()
		a.startRefreshListener()
		a.startFlashListener()
	}()

	return a.app.Run()
}

func (a *App) startRefreshLoop() {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				_ = a.vm.LoadChats(a.ctx)
				_ = a.vm.LoadSessionStatus(a.ctx)
				a.app.QueueUpdateDraw(func() {
					currentPage := a.pages.Current()
					if currentPage == "conversations" {
						a.convList.Update(a.vm.GetChats())
					}
					a.sessionInfo.Update(a.vm.GetSessionInfo())

					ss := a.vm.GetSessionStatus()
					if ss != nil {
						if currentPage == "auth" && ss.Status != wppv1.SessionStatus_SESSION_STATUS_AUTH_REQUIRED {
							a.convList.Update(a.vm.GetChats())
							a.pages.Reset("conversations")
							a.app.SetFocus(a.convList)
						}
					}
				})
			case <-a.ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func (a *App) startRefreshListener() {
	go func() {
		for {
			select {
			case <-a.vm.RefreshCh():
				a.app.QueueUpdateDraw(func() {
					currentPage := a.pages.Current()
					switch currentPage {
					case "conversations":
						a.convList.Update(a.vm.GetChats())
					case "messages":
						a.msgThread.Update(a.vm.GetMessages())
					}
					a.sessionInfo.Update(a.vm.GetSessionInfo())
				})
			case <-a.ctx.Done():
				return
			}
		}
	}()
}

func (a *App) startFlashListener() {
	go func() {
		for {
			select {
			case msg := <-a.vm.FlashUI.Watch():
				a.app.QueueUpdateDraw(func() {
					a.flashBar.Update(&msg)
				})
				// Auto-clear after expiry.
				go func(expires time.Time) {
					time.Sleep(time.Until(expires))
					a.app.QueueUpdateDraw(func() {
						a.flashBar.Update(a.vm.FlashUI.GetMessage())
					})
				}(msg.Expires)
			case <-a.ctx.Done():
				return
			}
		}
	}()
}

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
				go func() {
					_ = a.vm.LoadSessionStatus(a.ctx)
					_ = a.vm.LoadChats(a.ctx)
					a.app.QueueUpdateDraw(func() {
						a.convList.Update(a.vm.GetChats())
						a.sessionInfo.Update(a.vm.GetSessionInfo())
						a.pages.Reset("conversations")
						a.app.SetFocus(a.convList)
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
