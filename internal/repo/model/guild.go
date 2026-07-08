package model

type Guild struct {
	ID   string `gorm:"primaryKey"`
	Name string
}
