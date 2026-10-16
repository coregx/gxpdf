package layout

// FitTolerance is how far, in points, content may exceed the available
// height and still count as fitting.
//
// Heights reach an element through chains of float64 arithmetic: content
// plus padding, row heights grown by row-spanning cells, sums of rows minus
// padding. Content measured to fit exactly can therefore arrive a few ulps
// taller than the area derived from it, and an exact comparison would drop
// it. 0.01pt (1/7200 inch) is far below anything visible and far above the
// accumulated rounding of any realistic layout.
const FitTolerance = 0.01

// unlimitedHeight is the threshold from which an Area height means "no
// limit"; measuring passes use Height: 1e9.
const unlimitedHeight = 1e8

// Fits reports whether content of height need fits into the available height
// avail, allowing FitTolerance. An unlimited area always fits.
//
// Element implementations should use Fits for every "does this fit on the
// page" decision, so that all elements agree on what fits.
func Fits(need, avail float64) bool {
	return avail >= unlimitedHeight || need <= avail+FitTolerance
}
