package main

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	windowTitle        = "8-Puzzle (Go + Fyne)"
	windowWidth        = 560
	windowHeight       = 740
	labelTitle         = "8-Puzzle • A* con heurística seleccionable"
	labelHeuristic     = "Heurística:"
	statusReadyMessage = "Listo."
	statusResetMessage = "Estado reiniciado (meta)."
	statusAlreadyFinal = "Ya estás en el estado final."
	msgMixedFmt        = "Mezclado con %d pasos válidos."
	msgSolvedFmt       = "Solución en %d pasos • Nodos expandidos: %d"
	msgStepFmt         = "Paso %d / %d"

	buttonInitText    = "Iniciar"
	buttonShuffleText = "Mezclar"
	buttonSolveText   = "Resolver"
	buttonStepText    = "Paso"

	// UI grid 9×9
	uiGridSize = 9
	uiGridLen  = uiGridSize * uiGridSize

	// Tiles
	tileSize         = 54
	tileCornerRadius = 10
	tileFontSize     = 20

	// Mezcla
	shuffleStepsMin     = 0
	shuffleStepsMax     = 200
	defaultShuffleSteps = 30

	// Animación
	animateFrameMs = 140
)

// Colores en (hex)
const (
	colorBgDarkHex      = "#0f172a"
	colorBgLightHex     = "#f8fafc"
	colorFgDarkHex      = "#e5e7eb"
	colorFgLightHex     = "#0f172a"
	colorPrimaryHex     = "#22c55e"
	colorTileHex        = "#334155"
	colorTileBlankHex   = "#1f2937"
	colorPlaceholderHex = "#9ca3af"
)

type sleekTheme struct{}

func (sleekTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		if variant == theme.VariantLight {
			return mustHex(colorBgLightHex)
		}
		return mustHex(colorBgDarkHex)
	case theme.ColorNameForeground:
		if variant == theme.VariantLight {
			return mustHex(colorFgLightHex)
		}
		return mustHex(colorFgDarkHex)
	case theme.ColorNamePrimary:
		return mustHex(colorPrimaryHex)
	case theme.ColorNameButton:
		return mustHex(colorTileHex)
	case theme.ColorNameInputBackground:
		if variant == theme.VariantLight {
			return color.White
		}
		return mustHex(colorTileBlankHex)
	case theme.ColorNamePlaceHolder:
		return mustHex(colorPlaceholderHex)
	default:
		return theme.DefaultTheme().Color(name, variant)
	}
}

func (sleekTheme) Font(style fyne.TextStyle) fyne.Resource { return theme.DefaultTheme().Font(style) }
func (sleekTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}
func (sleekTheme) Size(name fyne.ThemeSizeName) float32 { return theme.DefaultTheme().Size(name) }

// Helpers de color
func mustHex(s string) color.Color {
	c, err := parseHexColor(s)
	if err != nil {
		return color.White
	}
	return c
}

func parseHexColor(s string) (color.NRGBA, error) {
	if len(s) != 7 || s[0] != '#' {
		return color.NRGBA{}, fmt.Errorf("invalid hex: %s", s)
	}
	var rr, gg, bb uint8
	if _, err := fmt.Sscanf(s, "#%02x%02x%02x", &rr, &gg, &bb); err != nil {
		return color.NRGBA{}, err
	}
	return color.NRGBA{R: rr, G: gg, B: bb, A: 255}, nil
}

// Tap area transparentes
type tapArea struct {
	widget.BaseWidget
	onTap func()
}

func newTapArea(onTap func()) *tapArea {
	t := &tapArea{onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}
func (t *tapArea) CreateRenderer() fyne.WidgetRenderer {
	rect := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	return widget.NewSimpleRenderer(rect)
}
func (t *tapArea) Tapped(*fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}
func (t *tapArea) TappedSecondary(*fyne.PointEvent) {}
func (t *tapArea) MinSize() fyne.Size               { return fyne.NewSize(tileSize, tileSize) }

// Componente Tile
type tile struct {
	background *canvas.Rectangle
	label      *canvas.Text
	wrapper    *fyne.Container
}

func newTile(onTap func()) *tile {
	bg := canvas.NewRectangle(mustHex(colorTileHex))
	bg.CornerRadius = tileCornerRadius
	lbl := canvas.NewText("", mustHex(colorFgDarkHex))
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	lbl.TextSize = tileFontSize

	center := container.NewCenter(lbl)
	tapper := newTapArea(onTap)
	wrap := container.NewMax(bg, center, tapper)

	return &tile{
		background: bg,
		label:      lbl,
		wrapper:    wrap,
	}
}

func (t *tile) setNumber(n int) {
	if n == BlankTile {
		t.label.Text = ""
		t.background.FillColor = mustHex(colorTileBlankHex)
	} else {
		t.label.Text = strconv.Itoa(n)
		t.background.FillColor = mustHex(colorTileHex)
	}
	t.label.Refresh()
	t.background.Refresh()
}

// ---------- Estado de UI ----------
type puzzleUI struct {
	window            fyne.Window
	tiles             [uiGridLen]*tile
	currentState      State
	solutionPath      []State
	stepIndex         int
	heuristicSelect   *widget.Select
	shuffleSlider     *widget.Slider
	shuffleValueLabel *widget.Label
	statusLabel       *widget.Label

	// animación
	isAnimating bool
	animCancel  chan struct{}

	// refs para deshabilitar
	btnInit    *widget.Button
	btnShuffle *widget.Button
	btnSolve   *widget.Button
	btnStep    *widget.Button
}

func main() {
	a := app.New()
	a.Settings().SetTheme(sleekTheme{})

	w := a.NewWindow(windowTitle)
	w.Resize(fyne.NewSize(windowWidth, windowHeight))

	ui := &puzzleUI{
		window:       w,
		currentState: Goal(),
		statusLabel:  widget.NewLabel(statusReadyMessage),
	}

	// Heurística
	heuristicOptions := []string{
		heuristicDisplayName[heuristicManhattan],
		heuristicDisplayName[heuristicMisplaced],
	}
	ui.heuristicSelect = widget.NewSelect(heuristicOptions, func(string) {})
	ui.heuristicSelect.SetSelected(heuristicOptions[0])

	// Slider mezcla
	ui.shuffleSlider = widget.NewSlider(shuffleStepsMin, shuffleStepsMax)
	ui.shuffleSlider.Step = 1
	ui.shuffleSlider.Value = defaultShuffleSteps
	ui.shuffleValueLabel = widget.NewLabel(strconv.Itoa(defaultShuffleSteps))
	ui.shuffleSlider.OnChanged = func(v float64) {
		ui.shuffleValueLabel.SetText(strconv.Itoa(int(math.Round(v))))
	}

	// Grid 9×9
	grid := ui.buildGrid()

	// Toolbar
	toolbar := widget.NewToolbar(
		widget.NewToolbarAction(theme.HomeIcon(), func() { ui.reset() }),
		widget.NewToolbarAction(theme.ViewRefreshIcon(), func() { ui.shuffle() }),
		widget.NewToolbarAction(theme.ConfirmIcon(), func() { ui.solveAnimated() }),
		widget.NewToolbarAction(theme.NavigateNextIcon(), func() { ui.step() }),
	)

	// Controles
	ui.btnInit = widget.NewButton(buttonInitText, func() { ui.reset() })
	ui.btnShuffle = widget.NewButton(buttonShuffleText, func() { ui.shuffle() })
	ui.btnSolve = widget.NewButton(buttonSolveText, func() { ui.solveAnimated() })
	ui.btnStep = widget.NewButton(buttonStepText, func() { ui.step() })

	controls := widget.NewCard("Controles", "",
		container.NewVBox(
			container.NewGridWithColumns(2,
				widget.NewLabel(labelHeuristic),
				ui.heuristicSelect,
			),
			widget.NewSeparator(),
			widget.NewLabel("Pasos a mezclar:"),
			container.NewBorder(nil, nil, nil, ui.shuffleValueLabel, ui.shuffleSlider),
			container.NewHBox(ui.btnInit, ui.btnShuffle, ui.btnSolve, ui.btnStep),
		),
	)

	// Título
	titleText := canvas.NewText(labelTitle, mustHex(colorFgDarkHex))
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	titleText.Alignment = fyne.TextAlignCenter
	titleBar := container.NewPadded(container.NewCenter(titleText))

	root := container.NewBorder(
		container.NewVBox(titleBar, toolbar),
		ui.statusLabel,
		nil,
		nil,
		container.NewVBox(grid, controls),
	)

	w.SetContent(container.NewPadded(root))
	ui.paint(ui.currentState)
	w.ShowAndRun()
}

// Acciones
func (ui *puzzleUI) reset() {
	ui.stopAnimation()
	ui.currentState = Goal()
	ui.solutionPath = nil
	ui.stepIndex = 0
	ui.paint(ui.currentState)
	ui.statusLabel.SetText(statusResetMessage)
}

func (ui *puzzleUI) shuffle() {
	ui.stopAnimation()
	steps := int(math.Round(ui.shuffleSlider.Value))
	state, err := ShuffleFromGoal(steps)
	if err != nil {
		dialog.ShowError(err, ui.window)
		return
	}
	ui.currentState = state
	ui.solutionPath = nil
	ui.stepIndex = 0
	ui.paint(ui.currentState)
	ui.statusLabel.SetText(fmt.Sprintf(msgMixedFmt, steps))
}

func (ui *puzzleUI) solveAnimated() {
	kind := ui.selectedHeuristic()
	result, err := Puzzle(ui.currentState, kind, defaultMaxExpand)
	if err != nil {
		dialog.ShowError(err, ui.window)
		return
	}
	if !result.found || len(result.path) == 0 {
		dialog.ShowError(errNoSolution, ui.window)
		return
	}

	ui.stopAnimation()
	ui.solutionPath = result.path
	ui.stepIndex = 0
	ui.disableControls(true)
	ui.isAnimating = true
	ui.animCancel = make(chan struct{})

	go func(path []State, expanded int) {
		ticker := time.NewTicker(time.Millisecond * animateFrameMs)
		defer ticker.Stop()

		total := len(path) - 1
		for ui.stepIndex < len(path) {
			select {
			case <-ui.animCancel:
				return
			case <-ticker.C:
				state := path[ui.stepIndex]

				ui.paint(state)
				ui.statusLabel.SetText(fmt.Sprintf(msgStepFmt, ui.stepIndex, total))
				ui.stepIndex++
			}
		}
		ui.statusLabel.SetText(fmt.Sprintf(msgSolvedFmt, total, expanded))
		ui.disableControls(false)
		ui.isAnimating = false
	}(result.path, result.expanded)
}

func (ui *puzzleUI) step() {
	if ui.isAnimating {
		ui.stopAnimation()
	}
	if len(ui.solutionPath) == 0 {
		kind := ui.selectedHeuristic()
		result, err := Puzzle(ui.currentState, kind, defaultMaxExpand)
		if err != nil {
			dialog.ShowError(err, ui.window)
			return
		}
		if !result.found || len(result.path) == 0 {
			dialog.ShowError(errNoSolution, ui.window)
			return
		}
		ui.solutionPath = result.path
		ui.stepIndex = 0
	}
	if ui.stepIndex >= len(ui.solutionPath) {
		ui.statusLabel.SetText(statusAlreadyFinal)
		return
	}
	nextState := ui.solutionPath[ui.stepIndex]
	ui.paint(nextState)
	ui.statusLabel.SetText(fmt.Sprintf(msgStepFmt, ui.stepIndex, len(ui.solutionPath)-1))
	ui.stepIndex++
}

// Animación helperss
func (ui *puzzleUI) stopAnimation() {
	if ui.isAnimating {
		if ui.animCancel != nil {
			close(ui.animCancel)
		}
		ui.animCancel = nil
		ui.isAnimating = false
		ui.disableControls(false)
	}
}

func (ui *puzzleUI) disableControls(disable bool) {
	if disable {
		ui.heuristicSelect.Disable()
		ui.btnInit.Disable()
		ui.btnShuffle.Disable()
		ui.btnSolve.Disable()
		ui.btnStep.Disable()
	} else {
		ui.heuristicSelect.Enable()
		ui.btnInit.Enable()
		ui.btnShuffle.Enable()
		ui.btnSolve.Enable()
		ui.btnStep.Enable()
	}
}

// Utilidades UI
func (ui *puzzleUI) buildGrid() *fyne.Container {
	objects := make([]fyne.CanvasObject, 0, uiGridLen)
	for i := 0; i < uiGridLen; i++ {
		t := newTile(func() { ui.step() }) // clic = avanzar paso
		ui.tiles[i] = t
		objects = append(objects, t.wrapper)
	}
	return container.NewGridWrap(fyne.NewSize(tileSize, tileSize), objects...)
}

// pinta el estado 3×3 centrado dentro de la tablerto 9×9
func (ui *puzzleUI) paint(state State) {
	centerStart := (uiGridSize - GridSize) / 2
	for r := 0; r < uiGridSize; r++ {
		for c := 0; c < uiGridSize; c++ {
			uiIdx := r*uiGridSize + c
			if r >= centerStart && r < centerStart+GridSize && c >= centerStart && c < centerStart+GridSize {
				sr := r - centerStart
				sc := c - centerStart
				stateIdx := sr*GridSize + sc
				ui.tiles[uiIdx].setNumber(state[stateIdx])
			} else {
				ui.tiles[uiIdx].setNumber(BlankTile)
			}
		}
	}
}

func (ui *puzzleUI) selectedHeuristic() Heuristic {
	switch ui.heuristicSelect.Selected {
	case heuristicDisplayName[heuristicMisplaced]:
		return heuristicMisplaced
	default:
		return heuristicManhattan
	}
}
