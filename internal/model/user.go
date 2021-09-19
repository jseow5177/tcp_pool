package model

type LoginUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginUserResponse struct {
	ErrCode uint32 `json:"err_code"`
	Message string `json:"message"`
}
