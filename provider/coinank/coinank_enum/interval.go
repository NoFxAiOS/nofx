package coinank_enum

type Interval string

const ( //not all interfaces support all interval
	Second1  Interval = "1s" // one second
	Second4  Interval = "4s"
	Second5  Interval = "5s"
	Second10 Interval = "10s"
	Second30 Interval = "30s"
	Minute1  Interval = "1m" // one minute
	Minute3  Interval = "3m"
	Minute5  Interval = "5m"
	Minute10 Interval = "10m"
	Minute15 Interval = "15m"
	Minute30 Interval = "30m"
	Hour1    Interval = "1h" // one hour
	Hour2    Interval = "2h"
	Hour4    Interval = "4h"
	Hour5    Interval = "5h"
	Hour6    Interval = "6h"
	Hour8    Interval = "8h"
	Hour12   Interval = "12h"
	Day1     Interval = "1d" // one day
	Day2     Interval = "2d"
	Day3     Interval = "3d"
	Day5     Interval = "5d"
	Week1    Interval = "1w" // one week
	Week2    Interval = "2w"
	Month1   Interval = "1M" // one month
	Month3   Interval = "3M"
	Month6   Interval = "6M"
	Year1    Interval = "1y" // one year
)
