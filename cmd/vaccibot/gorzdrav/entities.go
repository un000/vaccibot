package gorzdrav

type ResponseDistrict struct {
	Result    []*District `json:"result"`
	Success   bool        `json:"success"`
	ErrorCode int         `json:"errorCode"`
}

type ResponseLPU struct {
	Result    []*LPU `json:"result"`
	Success   bool   `json:"success"`
	ErrorCode int    `json:"errorCode"`
}

type ResponseSpecialty struct {
	Result    []*Specialty `json:"result"`
	Success   bool         `json:"success"`
	ErrorCode int          `json:"errorCode"`
}

type District struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Okato int    `json:"okato"`
}

type LPU struct {
	ID               int    `json:"id"`
	Description      string `json:"description"`
	DistrictID       int    `json:"districtId"`
	DistrictName     string `json:"districtName"`
	IsActive         bool   `json:"isActive"`
	LpuFullName      string `json:"lpuFullName"`
	LpuShortName     string `json:"lpuShortName"`
	Oid              string `json:"oid"`
	PartOf           int    `json:"partOf"`
	HeadOrganization string `json:"headOrganization"`
	Organization     string `json:"organization"`
	Address          string `json:"address"`
	Phone            string `json:"phone"`
	Email            string `json:"email"`
	Longitude        string `json:"longitude"`
	Latitide         string `json:"latitide"`
	CovidVaccination bool   `json:"covidVaccination"`

	District *District `json:"-"`
}

type Specialty struct {
	ID                   string  `json:"id"`
	FerID                string  `json:"ferId"`
	Name                 string  `json:"name"`
	CountFreeParticipant int     `json:"countFreeParticipant"`
	CountFreeTicket      int     `json:"countFreeTicket"`
	LastDate             *string `json:"lastDate"`
	NearestDate          *string `json:"nearestDate"`

	LPU *LPU `json:"-"`
}
