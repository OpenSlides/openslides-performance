package browser

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/OpenSlides/openslides-performance/client"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ostcar/topic"
	"golang.org/x/sync/errgroup"
)

func (o replay) Run(ctx context.Context, cfg client.Config) error {
	eg, ctx := errgroup.WithContext(ctx)

	app := tea.NewProgram(initialModel(o.Amount, o.Close), tea.WithContext(ctx), tea.WithoutSignalHandler())

	eg.Go(func() error {
		clients, err := o.loginUsers(ctx, cfg, app)
		if err != nil {
			return fmt.Errorf("login users: %w", err)
		}

		if err := multiBrowser(ctx, clients, app, o.Commands); err != nil {
			if !errors.Is(err, context.Canceled) {
				return fmt.Errorf("multi browser: %w", err)
			}
		}

		return nil
	})

	eg.Go(func() error {
		if _, err := app.Run(); err != nil {
			return fmt.Errorf("bubble tea app: %w", err)
		}

		return nil
	})

	if err := eg.Wait(); !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}

func (o replay) loginUsers(ctx context.Context, cfg client.Config, app *tea.Program) ([]*client.Client, error) {
	if o.UserTemplate == "" {
		cli, err := client.New(cfg)
		if err != nil {
			return nil, fmt.Errorf("create client: %w", err)
		}

		if err := cli.Login(ctx); err != nil {
			return nil, fmt.Errorf("login client: %w", err)
		}

		clients := make([]*client.Client, o.Amount)
		for i := 0; i < o.Amount; i++ {
			clients[i] = cli
		}

		app.Send(bMSGLogin)
		return clients, nil
	}

	eg, ctx := errgroup.WithContext(ctx)

	clients := make([]*client.Client, o.Amount)
	for i := 0; i < o.Amount; i++ {
		i := i

		eg.Go(func() error {
			username := strings.ReplaceAll(o.UserTemplate, "%i", strconv.Itoa(i+1))

			cli, err := client.New(cfg)
			if err != nil {
				return fmt.Errorf("create client for %s: %w", username, err)
			}

			if err := cli.LoginWithCredentials(ctx, username, cfg.Password); err != nil {
				return fmt.Errorf("login client for %s: %w", username, err)
			}

			clients[i] = cli
			app.Send(bMSGLogin)

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return clients, nil
}

type sender interface {
	Send(msg tea.Msg)
}

// multiBrowser simulates multiple browsers.
//
// r is a reader with controll lines, that tell each browsers what requests have
// to be sent.
//
// The function blocks until an error happens or the context get closed.
func multiBrowser(ctx context.Context, clients []*client.Client, send sender, r io.Reader) error {
	top := topic.New[string]()
	eg, ctx := errgroup.WithContext(ctx)
	for i := 0; i < len(clients); i++ {
		i := i
		eg.Go(func() error {
			if err := browser(ctx, clients[i], top, send); err != nil {
				return fmt.Errorf("browser %d failed: %w", i, err)
			}
			return nil
		})
	}

	eg.Go(func() error {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, prefix) {
				continue
			}

			line = strings.TrimSpace(strings.TrimPrefix(line, prefix))

			top.Publish(line)
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}

		return nil
	})

	return eg.Wait()
}

type browserMsg int

const (
	bMSGLogin browserMsg = iota
	bMSGConnect
	bMSGDisconnect
	bMSGClose
	bMSGError
)

func browser(ctx context.Context, cli *client.Client, top *topic.Topic[string], send sender) (err error) {
	defer func() {
		send.Send(bMSGClose)
	}()

	eg, ctx := errgroup.WithContext(ctx)
	defer func() {
		if err := eg.Wait(); err != nil {
			err = fmt.Errorf("connections: %w", err)
		}
	}()

	var id uint64
	for {
		newID, lines, err := top.Receive(ctx, id)
		if err != nil {
			return fmt.Errorf("read next lines: %w", err)
		}

		id = newID
		for _, line := range lines {
			parts := strings.Split(line, " ")
			method := parts[0]
			url := parts[1]
			var body io.Reader
			if len(parts) > 2 {
				body = strings.NewReader(parts[2])
			}

			req, err := http.NewRequestWithContext(ctx, method, url, body)
			if err != nil {
				return fmt.Errorf("creating request: %w", err)
			}

			eg.Go(func() error {
				send.Send(bMSGConnect)
				defer func() {
					send.Send(bMSGDisconnect)
				}()

				resp, err := cli.Do(req)
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return nil
					}
					send.Send(bMSGError)
					return err
				}

				io.ReadAll(resp.Body)
				resp.Body.Close()
				return nil
			})
		}
	}
}

// Bubble tea app

type multiBrowserModel struct {
	loggedIn    int
	currentConn int
	totalConn   int
	browsers    int
	errors      []error
	autoClose   bool
}

func initialModel(browsers int, autoclose bool) multiBrowserModel {
	return multiBrowserModel{
		browsers:  browsers,
		autoClose: autoclose,
	}
}

func (m multiBrowserModel) Init() tea.Cmd {
	return nil
}

func (m multiBrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m, tea.Quit
	case browserMsg:
		switch msg {
		case -1:
			return m, tea.Quit
		case bMSGLogin:
			m.loggedIn++
		case bMSGConnect:
			m.currentConn++
			m.totalConn++
		case bMSGDisconnect:
			m.currentConn--
			if m.autoClose && m.currentConn <= 0 {
				return m, tea.Quit
			}
		case bMSGClose:
			m.browsers--
		}
	case error:
		m.errors = append(m.errors, msg)
	}

	return m, nil
}

func (m multiBrowserModel) View() string {
	lastErrors := make([]string, 0, 10)
	for i := len(m.errors) - 1; i > len(m.errors)-10 && i >= 0; i-- {
		lastErrors = append(lastErrors, m.errors[i].Error())
	}
	return fmt.Sprintf(
		`Logged in: %d
Current Connections: %d
Total Connections: %d	
Browsers: %d
Errors: %d
Last Errors:%s`,
		m.loggedIn,
		m.currentConn,
		m.totalConn,
		m.browsers,
		len(m.errors),
		"\n"+strings.Join(lastErrors, "\n"),
	)
}
