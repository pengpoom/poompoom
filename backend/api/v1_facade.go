package api

func toPublicStatus(internal string) string {
	switch internal {
	case "queued":
		return "queued"
	case "succeeded":
		return "succeeded"
	case "failed":
		return "failed"
	case "cancelled":
		return "cancelled"
	case "running", "cancel_requested":
		return "running"
	default:
		return "running"
	}
}
