package models

// Quote represents the personal quote shown in the About Me section.
type Quote struct {
	Text   string `json:"text"`
	Author string `json:"author"`
}

// IconItem represents a hobby, skill, or tool entry with an icon identifier and sort order.
type IconItem struct {
	Name     string `json:"name"`
	IconName string `json:"icon_name"`
	Order    int    `json:"order"`
}

// AboutMe holds the main profile and biography metadata.
type AboutMe struct {
	ID              int        `json:"id"`
	FullName        string     `json:"full_name"`
	Headline        string     `json:"headline"`
	Bio             string     `json:"bio"`
	ProfileImageURL string     `json:"profile_image_url"`
	ResumeURL       string     `json:"resume_url"`
	ContactEmail    string     `json:"contact_email"`
	LinkedInURL     string     `json:"linkedin_url"`
	GitHubURL       string     `json:"github_url"`
	Hobbies         []IconItem `json:"hobbies"`
	Skillset        []IconItem `json:"skillset"`
	Tools           []IconItem `json:"tools"`
	Quote           *Quote     `json:"quote"`
	UpdatedAt       string     `json:"updated_at"`
}

// AboutMeUpdate supports partial updates from individual admin drawers (About Me, Skillset, Tools).
type AboutMeUpdate struct {
	FullName        *string     `json:"full_name"`
	Headline        *string     `json:"headline"`
	Bio             *string     `json:"bio"`
	ProfileImageURL *string     `json:"profile_image_url"`
	ResumeURL       *string     `json:"resume_url"`
	ContactEmail    *string     `json:"contact_email"`
	LinkedInURL     *string     `json:"linkedin_url"`
	GitHubURL       *string     `json:"github_url"`
	Hobbies         *[]IconItem `json:"hobbies"`
	Skillset        *[]IconItem `json:"skillset"`
	Tools           *[]IconItem `json:"tools"`
	Quote           *Quote      `json:"quote"`
	ClearQuote      bool        `json:"clear_quote,omitempty"`
}

// Experience represents a work or education timeline item.
type Experience struct {
	ID             string `json:"id"`
	JobTitle       string `json:"job_title"`
	CompanyName    string `json:"company_name"`
	CompanyLogoURL string `json:"company_logo_url"`
	Location       string `json:"location"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
	Description    string `json:"description"`
	IconName       string `json:"icon_name"`
	DisplayOrder   int    `json:"display_order"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// Certificate represents a certification or conference entry.
type Certificate struct {
	ID                  string `json:"id"`
	Title               string `json:"title"`
	IssuingOrganization string `json:"issuing_organization"`
	IssueDate           string `json:"issue_date"`
	CredentialURL       string `json:"credential_url"`
	CertificateFileURL  string `json:"certificate_file_url"`
	CreatedAt           string `json:"created_at"`
}

// Project represents a showcase project in the portfolio grid.
type Project struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	ImageURL       string   `json:"image_url"`
	ProjectLink    string   `json:"project_link"`
	SourceCodeLink string   `json:"source_code_link"`
	Technologies   []string `json:"technologies"`
	Featured       bool     `json:"featured"`
	Category       string   `json:"category"`
	Role           string   `json:"role"`
	Year           string   `json:"year"`
	Highlights     []string `json:"highlights"`
	DisplayOrder   int      `json:"display_order"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

// PortfolioData aggregates all database-backed portfolio sections for SSR.
type PortfolioData struct {
	About        AboutMe       `json:"about"`
	Experiences  []Experience  `json:"experiences"`
	Certificates []Certificate `json:"certificates"`
	Projects     []Project     `json:"projects"`
}
