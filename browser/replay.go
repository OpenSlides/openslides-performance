package browser

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/OpenSlides/openslides-performance/internal/client"
	"github.com/OpenSlides/openslides-performance/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ostcar/topic"
	"golang.org/x/sync/errgroup"
)

func (o replay) Run(ctx context.Context, cfg config.Config) error {
	app := tea.NewProgram(initialModel(o.Amount))

	go func() {
		defer o.Commands.Close()
		if err := multiBrowser(ctx, cfg, o.Amount, app, o.Commands); err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Println(fmt.Errorf("creating multi browser: %w", err))
				return
			}
		}

		return
	}()

	if err := app.Start(); err != nil {
		return fmt.Errorf("running bubble tea app: %w", err)
	}

	return nil
}

type sender interface {
	Send(msg tea.Msg)
}

// multiBrowser simulates multiple browsers.
//
// amout in the numbers of browsers to sumulate.
//
// r is a reader with controll lines, that tell each browsers what requests have
// to be sent.
//
// The function blocks until an error happens or the context get closed.
func multiBrowser(ctx context.Context, cfg config.Config, amount int, send sender, r io.Reader) error {
	cli, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	if err := cli.Login(ctx); err != nil {
		return fmt.Errorf("login client: %w", err)
	}

	top := topic.New[string]()
	eg, ctx := errgroup.WithContext(ctx)
	for i := 0; i < amount; i++ {
		nr := i
		eg.Go(func() error {
			if err := browser(ctx, cli, top, send); err != nil {
				return fmt.Errorf("browser %d failed: %w", nr, err)
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
	bMSGConnect browserMsg = iota
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
	currentConn int
	totalConn   int
	browsers    int
	errors      []error
}

func initialModel(browsers int) multiBrowserModel {
	return multiBrowserModel{
		browsers: browsers,
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
		case bMSGConnect:
			m.currentConn++
			m.totalConn++
		case bMSGDisconnect:
			m.currentConn--
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
		`Current Connections: %d
Total Connections: %d	
Browsers: %d
Errors: %d
Last Errors:%s`,
		m.currentConn,
		m.totalConn,
		m.browsers,
		len(m.errors),
		"\n"+strings.Join(lastErrors, "\n"),
	)
}
