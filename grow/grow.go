package grow

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Branch types — mirrors cbonsai's enum branchType
// ---------------------------------------------------------------------------

type branchType int

const (
	branchTrunk branchType = iota
	branchShootLeft
	branchShootRight
	branchDying
	branchDead
)

// ---------------------------------------------------------------------------
// Canvas
// ---------------------------------------------------------------------------

type pos struct{ y, x int }

type cell struct {
	ch    string
	style lipgloss.Style
}

// ---------------------------------------------------------------------------
// Growth queue entry
// ---------------------------------------------------------------------------

type shoot struct {
	x, y          int
	bType         branchType
	life          int
	shootCooldown int
}

// ---------------------------------------------------------------------------
// Bubbletea messages
// ---------------------------------------------------------------------------

type tickMsg struct{}

func tick() tea.Cmd {
	return tea.Tick(40*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

// ---------------------------------------------------------------------------
// Styles
// ---------------------------------------------------------------------------

var (
	styleTrunk      = lipgloss.NewStyle().Foreground(lipgloss.Color("94"))  // dark brown
	styleTrunkLight = lipgloss.NewStyle().Foreground(lipgloss.Color("136")) // golden
	styleLeaf       = lipgloss.NewStyle().Foreground(lipgloss.Color("34"))  // dark green
	styleLeafLight  = lipgloss.NewStyle().Foreground(lipgloss.Color("40"))  // bright green
	stylePot        = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // grey
)

// ---------------------------------------------------------------------------
// Config — matches cbonsai defaults
// ---------------------------------------------------------------------------

const (
	lifeStart  = 32
	multiplier = 5
	stepsPerTick = 3 // growth steps processed per tick
)

var leaves = []string{"●", "●", "✿", "●"}

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

type Model struct {
	width, height int
	canvas        map[pos]cell
	queue         []shoot
	shootCounter  int
	done          bool
	rng           *rand.Rand
}

func New(width, height int) Model {
	m := Model{
		width:  width,
		height: height,
		canvas: make(map[pos]cell),
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	// seed the trunk from the bottom-center
	m.queue = append(m.queue, shoot{
		x:    width / 2,
		y:    height - potHeight - 1,
		bType: branchTrunk,
		life: lifeStart,
	})
	return m
}

func (m Model) Init() tea.Cmd {
	return tick()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg:
		if m.done {
			return m, nil
		}
		for range stepsPerTick {
			if len(m.queue) == 0 {
				m.done = true
				break
			}
			var next []shoot
			for _, s := range m.queue {
				grown := m.grow(s)
				next = append(next, grown...)
			}
			m.queue = next
		}
		return m, tick()
	}
	return m, nil
}

func (m Model) View() string {
	var sb strings.Builder

	// build a row×col grid
	rows := make([][]string, m.height)
	for i := range rows {
		rows[i] = make([]string, m.width)
		for j := range rows[i] {
			rows[i][j] = " "
		}
	}

	for p, c := range m.canvas {
		if p.y >= 0 && p.y < m.height && p.x >= 0 && p.x < m.width {
			rows[p.y][p.x] = c.style.Render(c.ch)
		}
	}

	// render pot below tree
	potY := m.height - potHeight
	potX := m.width/2 - potWidth/2
	for dy, line := range potLines() {
		for dx, ch := range line {
			y, x := potY+dy, potX+dx
			if y >= 0 && y < m.height && x >= 0 && x < m.width {
				rows[y][x] = stylePot.Render(string(ch))
			}
		}
	}

	for i, row := range rows {
		sb.WriteString(strings.Join(row, ""))
		if i < len(rows)-1 {
			sb.WriteByte('\n')
		}
	}

	if m.done {
		sb.WriteString("\n\n  " + stylePot.Render("press any key to quit"))
	}

	return sb.String()
}

// ---------------------------------------------------------------------------
// Growth algorithm — ported from cbonsai
// ---------------------------------------------------------------------------

// grow advances one step for a shoot and returns child shoots spawned this step.
func (m *Model) grow(s shoot) []shoot {
	if s.life <= 0 {
		return nil
	}

	s.life--
	age := lifeStart - s.life

	dx, dy := m.setDeltas(s.bType, s.life, age, multiplier)
	ch, st := m.chooseChar(s.bType, dx, dy, s.life)

	nx, ny := s.x+dx, s.y+dy
	if ny >= 0 && ny < m.height-potHeight && nx >= 0 && nx < m.width {
		m.canvas[pos{ny, nx}] = cell{ch: ch, style: st}
	}

	s.x = nx
	s.y = ny

	if s.shootCooldown > 0 {
		s.shootCooldown--
	}

	var children []shoot

	// near-death: sprout dead branches
	if s.life < 3 {
		children = append(children, shoot{x: s.x, y: s.y, bType: branchDead, life: s.life})
		return children
	}

	// dying: sprout dying branches
	if s.bType == branchTrunk || s.bType == branchShootLeft || s.bType == branchShootRight {
		if s.life < multiplier+2 {
			children = append(children, shoot{x: s.x, y: s.y, bType: branchDying, life: s.life})
		}
	}

	// trunk branching
	if s.bType == branchTrunk {
		if age > 0 && (m.rng.Intn(3) == 0 || s.life%multiplier == 0) {
			if m.rng.Intn(8) == 0 {
				// inner trunk
				lifeAdj := s.life + m.rng.Intn(5) - 2
				children = append(children, shoot{x: s.x, y: s.y, bType: branchTrunk, life: lifeAdj})
			} else if s.shootCooldown <= 0 {
				// lateral shoot
				shootType := branchShootLeft
				if m.shootCounter%2 == 0 {
					shootType = branchShootRight
				}
				m.shootCounter++
				shootLife := s.life + multiplier - 2
				children = append(children, shoot{
					x: s.x, y: s.y,
					bType:         shootType,
					life:          shootLife,
					shootCooldown: multiplier * 2,
				})
				s.shootCooldown = multiplier * 2
			}
		}
	}

	// continue current branch
	if s.life > 0 {
		children = append(children, s)
	}

	return children
}

// setDeltas computes dx/dy for one growth step — ported from cbonsai setDeltas().
func (m *Model) setDeltas(bType branchType, life, age, mult int) (dx, dy int) {
	roll := m.rng.Intn(10)

	switch bType {
	case branchTrunk:
		if age <= 2 || life < 4 {
			dy = 0
			switch {
			case roll < 1:
				dx = -1
			case roll < 9:
				dx = 0
			default:
				dx = 1
			}
		} else if age < mult*3 {
			// young trunk: wider
			if mult == 5 {
				switch {
				case roll < 1:
					dy = -1
				case roll < 5:
					dy = 0
				default:
					dy = 0
				}
			}
			switch {
			case roll < 2:
				dx = -2
			case roll < 4:
				dx = -1
			case roll < 6:
				dx = 0
			case roll < 8:
				dx = 1
			default:
				dx = 2
			}
		} else {
			// mature trunk
			switch {
			case roll < 1:
				dy = -2
			case roll < 7:
				dy = -1
			default:
				dy = 0
			}
			switch {
			case roll < 2:
				dx = -1
			case roll < 8:
				dx = 0
			default:
				dx = 1
			}
		}

	case branchShootLeft:
		switch {
		case roll < 1:
			dy = -1
		case roll < 7:
			dy = 0
		default:
			dy = 1
		}
		switch {
		case roll < 4:
			dx = -2
		case roll < 7:
			dx = -1
		case roll < 9:
			dx = 0
		default:
			dx = 1
		}

	case branchShootRight:
		switch {
		case roll < 1:
			dy = -1
		case roll < 7:
			dy = 0
		default:
			dy = 1
		}
		switch {
		case roll < 1:
			dx = -1
		case roll < 3:
			dx = 0
		case roll < 7:
			dx = 1
		default:
			dx = 2
		}

	case branchDying:
		switch {
		case roll < 1:
			dy = -1
		case roll < 3:
			dy = 0
		default:
			dy = 1
		}
		switch {
		case roll < 2:
			dx = -2
		case roll < 4:
			dx = -1
		case roll < 6:
			dx = 0
		case roll < 8:
			dx = 1
		default:
			dx = 2
		}

	case branchDead:
		switch {
		case roll < 2:
			dy = -1
		case roll < 6:
			dy = 0
		default:
			dy = 1
		}
		switch {
		case roll < 2:
			dx = -2
		case roll < 4:
			dx = -1
		case roll < 6:
			dx = 0
		case roll < 8:
			dx = 1
		default:
			dx = 2
		}
	}

	return dx, dy
}

// chooseChar selects the character and style for a growth step — ported from cbonsai chooseString().
func (m *Model) chooseChar(bType branchType, dx, dy, life int) (string, lipgloss.Style) {
	useBold := m.rng.Intn(2) == 0

	trunkStyle := styleTrunk
	if useBold {
		trunkStyle = styleTrunkLight
	}
	leafStyle := styleLeaf
	if useBold {
		leafStyle = styleLeafLight
	}

	switch bType {
	case branchTrunk, branchShootLeft, branchShootRight:
		if dy < 0 {
			switch dx {
			case -2:
				return `\|`, trunkStyle
			case -1:
				return `\`, trunkStyle
			case 0:
				return `|`, trunkStyle
			case 1:
				return `/`, trunkStyle
			case 2:
				return `|/`, trunkStyle
			}
		} else if dy == 0 {
			switch dx {
			case -2:
				return `~~`, trunkStyle
			case -1:
				return `~`, trunkStyle
			case 0:
				return `|`, trunkStyle
			case 1:
				return `~`, trunkStyle
			case 2:
				return `~~`, trunkStyle
			}
		} else {
			switch dx {
			case -2:
				return `/~`, trunkStyle
			case -1:
				return `/`, trunkStyle
			case 0:
				return `|`, trunkStyle
			case 1:
				return `\`, trunkStyle
			case 2:
				return `~\`, trunkStyle
			}
		}

	case branchDying, branchDead:
		leaf := leaves[m.rng.Intn(len(leaves))]
		return leaf, leafStyle
	}

	return `|`, trunkStyle
}

// ---------------------------------------------------------------------------
// Pot
// ---------------------------------------------------------------------------

const (
	potHeight = 3
	potWidth  = 13
)

func potLines() []string {
	return []string{
		` ╭─────────╮ `,
		` │─────────│ `,
		` ╰─────────╯ `,
	}
}

// ---------------------------------------------------------------------------
// Run — entry point called from main
// ---------------------------------------------------------------------------

func Run() {
	p := tea.NewProgram(
		New(80, 40),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
	}
}
