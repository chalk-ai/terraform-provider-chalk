package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

func useStateForUnknownIncludingNull() planmodifier.String {
	return useStateForUnknownIncludingNullModifier{}
}

type useStateForUnknownIncludingNullModifier struct{}

func (m useStateForUnknownIncludingNullModifier) Description(_ context.Context) string {
	return "Once the resource exists, the value of this attribute in state will not change unless configured."
}

func (m useStateForUnknownIncludingNullModifier) MarkdownDescription(_ context.Context) string {
	return "Once the resource exists, the value of this attribute in state will not change unless configured."
}

func (m useStateForUnknownIncludingNullModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.State.Raw.IsNull() {
		return
	}

	if !req.PlanValue.IsUnknown() {
		return
	}

	if req.ConfigValue.IsUnknown() {
		return
	}

	resp.PlanValue = req.StateValue
}
