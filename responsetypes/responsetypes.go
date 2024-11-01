package responsetypes 

type PaginatedResponse[T any] struct {
	Page         int  `json:"page"`
	Size         int  `json:"size"`
	//TotalPages   int  `json:"total_pages"`
	//TotalObjects int  `json:"total_objects"`
	Data         []T `json:"data"`
}

