package db

type Host struct {
	MAC  string `db:"mac" json:"mac" binding:"required"`
	Ip   string `db:"ip" json:"ip" binding:"required"`
	Name string `db:"name" json:"name" binding:"required"`
}
