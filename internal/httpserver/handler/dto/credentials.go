package dto

// CredentialsRequest — тело запроса для POST /api/user/register и POST /api/user/login.
type CredentialsRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}
