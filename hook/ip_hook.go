package hook

import "github.com/rs/zerolog/log"

type IpResult struct {
	Err error
	IP  string
}

func (r *IpResult) Error() error {
	return r.Err
}

func (r *IpResult) GetResult() string {
	if r.Err != nil {
		log.Printf("⚠️ 执行GetIP时出错: %v", r.Err)
	}
	return r.IP
}
