package provider

import (
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
)

// validateWholeSecondDuration parses a duration attribute and rejects values
// that cannot be represented exactly by APIs that store durations in seconds.
func validateWholeSecondDuration(
	value timetypes.GoDuration,
	attributePath path.Path,
	attributeName string,
	diags *diag.Diagnostics,
) (time.Duration, bool) {
	if value.IsNull() || value.IsUnknown() {
		return 0, false
	}

	duration, parseDiags := value.ValueGoDuration()
	diags.Append(parseDiags...)
	if parseDiags.HasError() {
		return 0, false
	}
	if duration%time.Second != 0 {
		diags.AddAttributeError(
			attributePath,
			"Invalid Duration Precision",
			fmt.Sprintf("%s must resolve to a whole number of seconds.", attributeName),
		)
		return 0, false
	}

	return duration, true
}
