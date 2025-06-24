package utils

type PublicKeyShare struct {
	ParticipantID int    `json:"participant_id"`
	ShareData     string `json:"share_data"`
}

type SecretKeyShare struct {
	ParticipantID int    `json:"participant_id"`
	ShareData     string `json:"share_data"`
}
