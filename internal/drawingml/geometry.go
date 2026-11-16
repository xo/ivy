package drawingml

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/okage/roxmltree"
)

// The limits of a geometry. ParseCustom and ParseAdjust return an error that
// wraps ivy.ErrLimit for a geometry over one of them. The largest preset has
// 244 guides and 70 path commands.
const (
	// maxGuides is the number of guides of a geometry, with its adjust
	// values.
	maxGuides = 2048
	// maxPaths is the number of paths of a geometry.
	maxPaths = 512
	// maxCommands is the number of path commands of a geometry, in all of its
	// paths.
	maxCommands = 8192
)

// Geometry is the geometry of a shape: its adjust values, its guides, its
// paths and its text rectangle. A preset shape and a:custGeom have the same
// structure. A Geometry does not change after it is made, and it is safe for
// concurrent use.
type Geometry struct {
	// name is the name of a preset shape, or empty for a custom geometry.
	name   string
	adjust []Guide
	guides []Guide
	paths  []pathDef
	// text is the text rectangle. An empty L means the box of the shape.
	text textRect
}

// Guide is a named formula: an adjust value or a guide.
type Guide struct {
	name string
	op   op
	args [3]string
}

// op is an operator of a guide formula, from ST_GeomGuideFormula of
// ECMA-376 Part 1, 20.1.10.56.
type op uint8

// The operators of the guide formulas. Each one takes the number of
// arguments of ops.
const (
	opVal    op = iota // val x: x
	opMulDiv           // */ x y z: x * y / z
	opAddSub           // +- x y z: x + y - z
	opAddDiv           // +/ x y z: (x + y) / z
	opIfElse           // ?: x y z: y if x > 0, otherwise z
	opAbs              // abs x: the absolute value of x
	opAt2              // at2 x y: the angle of the point (x, y)
	opCat2             // cat2 x y z: x * cos(at2(y, z))
	opCos              // cos x y: x * cos(y)
	opMax              // max x y: the larger of x and y
	opMin              // min x y: the smaller of x and y
	opMod              // mod x y z: sqrt(x*x + y*y + z*z)
	opPin              // pin x y z: y, limited to the range from x to z
	opSat2             // sat2 x y z: x * sin(at2(y, z))
	opSin              // sin x y: x * sin(y)
	opSqrt             // sqrt x: the square root of x
	opTan              // tan x y: x * tan(y)
)

// opInfo is the name and the number of arguments of an operator.
type opInfo struct {
	name string
	n    int
}

// ops holds the name and the number of arguments of each operator.
var ops = [...]opInfo{
	opVal:    {"val", 1},
	opMulDiv: {"*/", 3},
	opAddSub: {"+-", 3},
	opAddDiv: {"+/", 3},
	opIfElse: {"?:", 3},
	opAbs:    {"abs", 1},
	opAt2:    {"at2", 2},
	opCat2:   {"cat2", 3},
	opCos:    {"cos", 2},
	opMax:    {"max", 2},
	opMin:    {"min", 2},
	opMod:    {"mod", 3},
	opPin:    {"pin", 3},
	opSat2:   {"sat2", 3},
	opSin:    {"sin", 2},
	opSqrt:   {"sqrt", 1},
	opTan:    {"tan", 2},
}

// pathDef is a path of a geometry, from a:path.
type pathDef struct {
	// w and h are the size of the coordinate space of the path. Zero means
	// the size of the shape.
	w, h      float64
	fill      Fill
	stroke    bool
	extrusion bool
	commands  []command
}

// command is a command of a path. The arguments are guide names or numbers.
type command struct {
	verb verb
	// args holds x and y for moveTo and lnTo, two points for quadBezTo,
	// three points for cubicBezTo, and wR, hR, stAng and swAng for arcTo.
	args [6]string
}

// verb is the kind of a path command.
type verb uint8

// The kinds of path commands.
const (
	verbMoveTo verb = iota
	verbLineTo
	verbArcTo
	verbQuadTo
	verbCubicTo
	verbClose
)

// textRect is the text rectangle of a geometry, from a:rect. Each field is a
// guide name or a number.
type textRect struct {
	l, t, r, b string
}

// Fill is the fill mode of a path. A mode other than FillNorm and FillNone
// fills the path with a lighter or a darker color than the fill of the shape.
type Fill uint8

// The fill modes, from ST_PathFillMode of ECMA-376 Part 1, 20.1.10.37.
const (
	FillNorm Fill = iota
	FillNone
	FillLighten
	FillLightenLess
	FillDarken
	FillDarkenLess
)

// fills holds the name of each fill mode.
var fills = [...]string{
	FillNorm:        "norm",
	FillNone:        "none",
	FillLighten:     "lighten",
	FillLightenLess: "lightenLess",
	FillDarken:      "darken",
	FillDarkenLess:  "darkenLess",
}

// String satisfies the fmt.Stringer interface. It returns the name of the
// mode in DrawingML, such as "lightenLess".
func (f Fill) String() string {
	if int(f) < len(fills) {
		return fills[f]
	}
	return "Fill(" + strconv.Itoa(int(f)) + ")"
}

// Preset returns the geometry of the preset shape with the name, such as
// "rect" or "star5". The name is a value of ST_ShapeType.
func Preset(name string) (*Geometry, bool) {
	i, ok := slices.BinarySearchFunc(presets[:], name, func(g Geometry, name string) int {
		return strings.Compare(g.name, name)
	})
	if !ok {
		return nil, false
	}
	return &presets[i], true
}

// AdjustNames returns the names of the adjust values of the geometry, in
// order, such as "adj1" and "adj2".
func (g *Geometry) AdjustNames() []string {
	names := make([]string, len(g.adjust))
	for i, a := range g.adjust {
		names[i] = a.name
	}
	return names
}

// AdjustValue returns the adjust value with the name and the value v, for
// Evaluate.
func AdjustValue(name string, v float64) Guide {
	return Guide{name: name, op: opVal, args: [3]string{strconv.FormatFloat(v, 'f', -1, 64)}}
}

// ParseCustom reads a custom geometry from the element n, which is
// a:custGeom. It reads the children avLst, gdLst, rect and pathLst, and it
// skips the others. It also reads a definition of presetShapeDefinitions.xml,
// which has the same children.
func ParseCustom(n roxmltree.Node) (*Geometry, error) {
	g := new(Geometry)
	commands := 0
	for c := range n.Children() {
		if !c.IsElement() {
			continue
		}
		var err error
		switch c.TagName().Name() {
		case "avLst":
			g.adjust, err = parseGuides(c, g.adjust, len(g.guides))
		case "gdLst":
			g.guides, err = parseGuides(c, g.guides, len(g.adjust))
		case "rect":
			g.text = textRect{
				l: attr(c, "l", "l"),
				t: attr(c, "t", "t"),
				r: attr(c, "r", "r"),
				b: attr(c, "b", "b"),
			}
		case "pathLst":
			err = g.parsePaths(c, &commands)
		}
		if err != nil {
			return nil, err
		}
	}
	return g, nil
}

// ParseAdjust reads the adjust values of a shape from the element n, which is
// a:avLst. Evaluate takes them, and they replace the adjust values of the
// geometry with the same names.
func ParseAdjust(n roxmltree.Node) ([]Guide, error) {
	return parseGuides(n, nil, 0)
}

// parseGuides appends the guides of the element n, a:avLst or a:gdLst, to
// list. others is the number of the other guides of the geometry.
func parseGuides(n roxmltree.Node, list []Guide, others int) ([]Guide, error) {
	for c := range n.Children() {
		if !c.IsElement() || c.TagName().Name() != "gd" {
			continue
		}
		if len(list)+others >= maxGuides {
			return nil, fmt.Errorf("drawingml: more than %d guides: %w", maxGuides, ivy.ErrLimit)
		}
		name, _ := c.Attribute(roxmltree.ExpandedNameFromStr("name"))
		fmla, _ := c.Attribute(roxmltree.ExpandedNameFromStr("fmla"))
		g, err := parseFormula(name, fmla)
		if err != nil {
			return nil, err
		}
		list = append(list, g)
	}
	return list, nil
}

// parseFormula returns the guide with the name and the formula s, such as
// "*/ w adj1 100000".
func parseFormula(name, s string) (Guide, error) {
	if name == "" {
		return Guide{}, fmt.Errorf("drawingml: a guide has no name: %w", ivy.ErrFormat)
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return Guide{}, fmt.Errorf("drawingml: guide %q has no formula: %w", name, ivy.ErrFormat)
	}
	g := Guide{name: name}
	i := slices.IndexFunc(ops[:], func(o opInfo) bool {
		return o.name == fields[0]
	})
	if i < 0 {
		return Guide{}, fmt.Errorf("drawingml: guide %q has the unknown operator %q: %w", name, fields[0], ivy.ErrFormat)
	}
	g.op = op(i)
	// A formula can have more arguments than its operator takes, and the
	// formula skips them. Eight guides of circularArrow,
	// leftCircularArrow and leftRightCircularArrow in ECMA-376 have
	// one more argument, such as "+- xH 0 dxB 0".
	if len(fields)-1 < ops[i].n {
		return Guide{}, fmt.Errorf("drawingml: guide %q has %d arguments, not %d: %w", name, len(fields)-1, ops[i].n, ivy.ErrFormat)
	}
	copy(g.args[:ops[i].n], fields[1:])
	return g, nil
}

// parsePaths reads the paths of the element n, a:pathLst. commands counts
// the path commands of the geometry.
func (g *Geometry) parsePaths(n roxmltree.Node, commands *int) error {
	for c := range n.Children() {
		if !c.IsElement() || c.TagName().Name() != "path" {
			continue
		}
		if len(g.paths) >= maxPaths {
			return fmt.Errorf("drawingml: more than %d paths: %w", maxPaths, ivy.ErrLimit)
		}
		p := pathDef{
			w:         length(c, "w"),
			h:         length(c, "h"),
			stroke:    boolean(c, "stroke", true),
			extrusion: boolean(c, "extrusionOk", true),
		}
		if s, ok := c.Attribute(roxmltree.ExpandedNameFromStr("fill")); ok {
			i := slices.Index(fills[:], s)
			if i < 0 {
				return fmt.Errorf("drawingml: unknown fill mode %q: %w", s, ivy.ErrFormat)
			}
			p.fill = Fill(i)
		}
		for e := range c.Children() {
			if !e.IsElement() {
				continue
			}
			cmd, ok, err := parseCommand(e)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			if *commands >= maxCommands {
				return fmt.Errorf("drawingml: more than %d path commands: %w", maxCommands, ivy.ErrLimit)
			}
			*commands++
			p.commands = append(p.commands, cmd)
		}
		g.paths = append(g.paths, p)
	}
	return nil
}

// parseCommand reads the path command of the element e. It returns false for
// an element that is not a path command, such as a:extLst.
func parseCommand(e roxmltree.Node) (command, bool, error) {
	var cmd command
	pts := 0
	switch e.TagName().Name() {
	case "moveTo":
		cmd.verb, pts = verbMoveTo, 1
	case "lnTo":
		cmd.verb, pts = verbLineTo, 1
	case "quadBezTo":
		cmd.verb, pts = verbQuadTo, 2
	case "cubicBezTo":
		cmd.verb, pts = verbCubicTo, 3
	case "arcTo":
		cmd.verb = verbArcTo
		cmd.args[0] = attr(e, "wR", "0")
		cmd.args[1] = attr(e, "hR", "0")
		cmd.args[2] = attr(e, "stAng", "0")
		cmd.args[3] = attr(e, "swAng", "0")
		return cmd, true, nil
	case "close":
		cmd.verb = verbClose
		return cmd, true, nil
	default:
		return cmd, false, nil
	}
	i := 0
	for pt := range e.Children() {
		if !pt.IsElement() || pt.TagName().Name() != "pt" {
			continue
		}
		if i == pts {
			break
		}
		cmd.args[2*i] = attr(pt, "x", "0")
		cmd.args[2*i+1] = attr(pt, "y", "0")
		i++
	}
	if i != pts {
		return cmd, false, fmt.Errorf("drawingml: %s has %d points, not %d: %w", e.TagName().Name(), i, pts, ivy.ErrFormat)
	}
	return cmd, true, nil
}

// attr returns the attribute of n with the name, or def if n has none.
func attr(n roxmltree.Node, name, def string) string {
	if s, ok := n.Attribute(roxmltree.ExpandedNameFromStr(name)); ok {
		return strings.TrimSpace(s)
	}
	return def
}

// length returns the attribute of n with the name as a length, or zero if n
// has none, it is not a number or it is less than 1. A length is a whole
// number of ST_PositiveCoordinate, so a smaller one is zero.
func length(n roxmltree.Node, name string) float64 {
	v, ok := number(attr(n, name, ""))
	if !ok || v < 1 {
		return 0
	}
	return v
}

// boolean returns the attribute of n with the name as an xsd:boolean, or def
// if n has none or it is not a boolean.
func boolean(n roxmltree.Node, name string, def bool) bool {
	switch attr(n, name, "") {
	case "true", "1":
		return true
	case "false", "0":
		return false
	}
	return def
}
