package util

import "github.com/google/uuid"

func UUID(uid string) uuid.UUID {
	uuid, _ := uuid.Parse(uid)
	return uuid
}
