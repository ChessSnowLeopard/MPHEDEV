package utils

type ParticipantInfo struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

type PublicKeyShare struct {
	ParticipantID int    `json:"participant_id"`
	ShareData     string `json:"share_data"`
}

type SecretKeyShare struct {
	ParticipantID int    `json:"participant_id"`
	ShareData     string `json:"share_data"`
}
