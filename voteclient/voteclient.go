package voteclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/OpenSlides/openslides-performance/internal/client"
	"github.com/OpenSlides/openslides-performance/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

// Run runs the command.
func (o Options) Run(ctx context.Context, cfg config.Config) error {
	cli, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("initial http client: %w", err)
	}

	p := tea.NewProgram(initialModel(o.PollID, cli))
	if err := p.Start(); err != nil {
		return fmt.Errorf("running bubble tea app: %w", err)
	}

	return nil
}

type model struct {
	pollID int

	ticks int
	err   error

	user user
	poll poll

	hasVoted bool
	ballot   ballot

	// Non model stuff
	client *client.Client
}

type user struct {
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Title     string `json:"title"`
}

func (u user) String() string {
	// Code from autoupdate projector.
	parts := func(sp ...string) []string {
		var full []string
		for _, s := range sp {
			if s == "" {
				continue
			}
			full = append(full, s)
		}
		return full
	}(u.FirstName, u.LastName)

	if len(parts) == 0 {
		parts = append(parts, u.Username)
	} else if u.Title != "" {
		parts = append([]string{u.Title}, parts...)
	}
	return strings.Join(parts, " ")
}

type poll struct {
	ID             int    `json:"id"`
	Title          string `json:"title"`
	Type           string `json:"type"`
	Method         string `json:"pollmethod"`
	State          string `json:"state"`
	MinVotes       int    `json:"min_votes_amount"`
	MaxVotes       int    `json:"max_votes_amount"`
	MaxVotesOption int    `json:"max_votes_per_option"`
	GlobalYes      bool   `json:"global_yes"`
	GlobalNo       bool   `json:"global_no"`
	GlobalAbstain  bool   `json:"global_abstain"`
	OptionIDs      []int  `json:"option_ids"`
}

type ballot struct {
	optionID int
	selected int
	err      error
	sending  bool
}

func initialModel(pollID int, client *client.Client) model {
	return model{
		pollID: pollID,
		client: client,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tick, login(m.client))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up":
			m.ballot.selected--
			return m, nil
		case "down":
			m.ballot.selected++
			return m, nil

		case "enter":

			m.ballot.sending = true
			m.ballot.err = nil
			voteValue := createVote(m.poll, m.ballot)
			return m, vote(m.client, m.pollID, voteValue)
		}

	case msgTick:
		m.ticks++
		return m, tick

	case msgLogin:
		if err := msg.err; err != nil {
			m.err = fmt.Errorf("login: %w", err)
			return m, nil
		}

		cmdAU := autoupdateConnect(m.client, autoupdateRequest(m.client.UserID(), m.pollID))
		cmdVoted := haveIVoted(m.client, m.pollID)

		return m, tea.Batch(cmdAU, cmdVoted)

	case msgAutoupdate:
		if err := msg.err; err != nil {
			m.err = fmt.Errorf("autoupdate: %w", err)
			return m, nil
		}

		if err := parseKV("user", m.client.UserID(), msg.value, &m.user); err != nil {
			m.err = fmt.Errorf("parsing user: %w", err)
			return m, nil
		}

		oldState := m.poll.State

		if err := parseKV("poll", m.pollID, msg.value, &m.poll); err != nil {
			m.err = fmt.Errorf("parsing poll: %w", err)
			return m, nil
		}

		if len(m.poll.OptionIDs) == 0 {
			m.err = fmt.Errorf("Poll has no options")
			return m, nil
		}

		m.ballot.optionID = m.poll.OptionIDs[0]

		var cmds []tea.Cmd
		if oldState != m.poll.State {
			cmds = append(cmds, haveIVoted(m.client, m.pollID))
		}

		if !msg.finished {
			cmds = append(cmds, msg.next)
		}

		return m, tea.Batch(cmds...)

	case msgHaveIVoted:
		if err := msg.err; err != nil {
			m.err = fmt.Errorf("have i voted: %w", err)
			return m, nil
		}

		m.hasVoted = msg.voted
		return m, nil

	case msgVote:
		m.ballot.sending = false
		if err := msg.err; err != nil {
			m.ballot.err = fmt.Errorf("sending ballot: %w", err)
			return m, nil
		}

		m.hasVoted = true
		return m, nil
	}

	return m, nil
}

func createVote(poll poll, ballot ballot) string {
	var v string
	switch ballot.selected % 3 {
	case 0:
		v = "Y"
	case 1:
		v = "N"
	case 2:
		v = "A"
	}

	value := fmt.Sprintf(`{"%d":"%s"}`, ballot.optionID, v)

	if poll.Type == "crypt" {
		// DO the magic
	}

	return value
}

func (m model) View() string {
	if m.err != nil {
		var errStatus client.HTTPStatusError
		if errors.As(m.err, &errStatus) && errStatus.StatusCode == 403 {
			var loginMsg struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(errStatus.Body, &loginMsg); err != nil {
				loginMsg.Message = string(errStatus.Body)
			}
			return fmt.Sprintf("Login impossible: %s", loginMsg.Message)
		}

		return fmt.Sprintf("Error: %v", m.err.Error())
	}

	userID := m.client.UserID()
	if userID == 0 {
		return fmt.Sprintf("Loggin in %s", viewProgress(m.ticks))
	}

	if m.user.Username == "" {
		return fmt.Sprintf("Logged in as user %d. Loading data %s", m.client.UserID(), viewProgress(m.ticks))
	}

	return fmt.Sprintf("Hello %s!\n\n%s\n", m.user, viewPoll(m.poll, m.ticks, m.hasVoted, m.ballot))
}

func viewProgress(ticks int) string {
	return strings.Repeat(".", (ticks%3)+1)
}

func viewPoll(poll poll, ticks int, hasVoted bool, ballot ballot) string {
	if poll.ID == 0 {
		return fmt.Sprintf("The poll does currently not exist. Please wait %s", viewProgress(ticks))
	}

	content := new(bytes.Buffer)

	fmt.Fprintf(content, "Poll: %s (%s)\n\n", poll.Title, poll.State)

	if poll.State != "started" {
		return content.String()
	}

	if ballot.err != nil {
		fmt.Fprintf(content, "Error: %v\n", ballot.err)
	}

	if hasVoted {
		fmt.Fprintf(content, "You already voted for poll %d\n", poll.ID)
		return content.String()
	}

	if ballot.sending {
		fmt.Fprintf(content, "Sending ballot %s\n", viewProgress(ticks))
	}

	if poll.Method != "YNA" {
		fmt.Fprintf(content, "Poll has type %s. This is not yet supported\n", poll.Type)
		return content.String()
	}

	if ballot.selected < 0 {
		ballot.selected += 3000
	}
	ballot.selected = ballot.selected % 3
	checked := map[bool]string{
		true:  "X",
		false: " ",
	}
	fmt.Fprintf(content, "[%s] Yes\n[%s] No\n[%s]Abstain\n", checked[ballot.selected == 0], checked[ballot.selected == 1], checked[ballot.selected == 2])

	return content.String()
}
