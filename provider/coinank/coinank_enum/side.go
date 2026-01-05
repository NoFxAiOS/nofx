package coinank_enum

type Side string

const (
	To   Side = "to"   //search backward from the time ts
	From Side = "from" //search forward from time ts
)
