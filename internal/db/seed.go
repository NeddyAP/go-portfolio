package db

import (
	"context"
	"encoding/json"

	"github.com/NeddyAP/go-portfolio/internal/models"
)

// SeedIfEmpty populates SQLite with all current live portfolio content from neddyap.tech
// (with typos cleaned up and local optimized static assets) if about_me is empty.
func (s *Store) SeedIfEmpty(ctx context.Context) error {
	var count int
	if err := s.sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM about_me`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	tx, err := s.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	now := nowISO()

	hobbies, _ := json.Marshal([]models.IconItem{
		{Name: "Music", IconName: "Music", Order: 0},
		{Name: "Reading Comic", IconName: "Book", Order: 1},
		{Name: "Playing Games", IconName: "Gamepad", Order: 2},
	})
	skillset, _ := json.Marshal([]models.IconItem{
		{Name: "PHP", IconName: "SiPhp", Order: 0},
		{Name: "Laravel", IconName: "FaLaravel", Order: 1},
		{Name: "JavaScript", IconName: "BiLogoJavascript", Order: 2},
		{Name: "Tailwind CSS", IconName: "RiTailwindCssFill", Order: 3},
		{Name: "React", IconName: "FaReact", Order: 4},
		{Name: "Git", IconName: "DiGit", Order: 5},
		{Name: "MySQL", IconName: "SiMysql", Order: 6},
		{Name: "PostgreSQL", IconName: "SiPostgresql", Order: 7},
	})
	tools, _ := json.Marshal([]models.IconItem{
		{Name: "VS Code", IconName: "BiLogoVisualStudio", Order: 0},
		{Name: "Postman", IconName: "SiPostman", Order: 1},
		{Name: "JetBrains", IconName: "SiJetbrains", Order: 2},
		{Name: "Windows", IconName: "DiWindows", Order: 3},
		{Name: "Linux", IconName: "AiOutlineLinux", Order: 4},
		{Name: "Docker", IconName: "SiDocker", Order: 5},
		{Name: "GitHub", IconName: "SiGithub", Order: 6},
	})
	quote, _ := json.Marshal(models.Quote{
		Text:   "Simple But Better!",
		Author: "Neddy",
	})

	_, err = tx.ExecContext(ctx, `
INSERT INTO about_me (id, full_name, headline, bio, profile_image_url, resume_url, contact_email, linkedin_url, github_url, hobbies_json, skillset_json, tools_json, quote_json, updated_at)
VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Neddy Avgha Prasetio",
		"Full-Stack Software Engineer",
		"Software Engineer with hands-on experience in developing internal Enterprise Resource Planning (ERP) systems, operational management applications, and web solutions using Laravel and React.js. Proficient in PHP, JavaScript, MySQL, Git, and server-side application deployment. Experienced in translating business requirements into functional software solutions, building dynamic admin panels, and providing Microsoft 365 technical support for large-scale enterprise projects.",
		"/static/images/profile.webp",
		"/static/files/resume.pdf",
		"neddy.prasetio@gmail.com",
		"https://www.linkedin.com/in/neddy-avgha-prasetio/",
		"https://github.com/neddyap",
		string(hobbies),
		string(skillset),
		string(tools),
		string(quote),
		now,
	)
	if err != nil {
		return err
	}

	experiences := []models.Experience{
		{
			ID:           "fb1addfd-3177-4622-805d-64c4d43e163c",
			JobTitle:     "Fullstack Developer",
			CompanyName:  "PT Kahaptex Textile",
			Location:     "Bogor, Jawa Barat",
			StartDate:    "2026-07-28",
			EndDate:      "",
			Description:  "Contributed to the development and implementation of an Enterprise Resource Planning (ERP) system to streamline business processes, improve operational efficiency, and support the company's digital transformation.",
			IconName:     "LuBriefcase",
			DisplayOrder: 1,
		},
		{
			ID:           "34e95f60-4c4a-41e6-b110-a574b327d1a9",
			JobTitle:     "Computer Science",
			CompanyName:  "Djuanda University",
			Location:     "Bogor, Jawa Barat",
			StartDate:    "2022-09-19",
			EndDate:      "2026-04-29",
			Description:  "Graduated with a Bachelor's degree in Information Technology at Djuanda University. Built SIMKAN, a KKL & KKN Management Workflow System for the Computer Science Faculty at Djuanda University as my thesis project.",
			IconName:     "LuGraduationCap",
			DisplayOrder: 2,
		},
		{
			ID:           "0732ff0f-af0c-4c94-8baa-fdac5c6fa821",
			JobTitle:     "Fullstack Developer - Internship",
			CompanyName:  "Dinas Perhubungan Kota Bogor",
			Location:     "Bogor, Jawa Barat",
			StartDate:    "2024-07-21",
			EndDate:      "2024-08-21",
			Description:  "Engage with internal teams and conduct data cleansing on internal vehicle data to support application development. This effort culminates in the development of a Vehicle Maintenance Management Website, utilizing PHP with the Laravel framework and a MySQL database.",
			IconName:     "LuBriefcase",
			DisplayOrder: 3,
		},
		{
			ID:           "1e711d32-22c2-4d19-80b7-93aa53de4531",
			JobTitle:     "Technical Support",
			CompanyName:  "PT. Helios Informatika Nusantara",
			Location:     "",
			StartDate:    "2021-01-01",
			EndDate:      "2021-08-31",
			Description:  "Collaborated with internal client teams to create documentation for the pre-activation and post-activation stages of MFA. Provided troubleshooting support for Microsoft 365 accounts in client projects and successfully activated MFA for over 16,000 accounts. The project concluded with the activation and troubleshooting of Microsoft 365 accounts for Bank Mandiri.",
			IconName:     "MdWorkOutline",
			DisplayOrder: 4,
		},
		{
			ID:           "624af441-f4ff-4b56-a1ff-5fccb4f70e3b",
			JobTitle:     "Junior Programmer - Internship",
			CompanyName:  "PT. Nawa Darsana Teknologi",
			Location:     "",
			StartDate:    "2019-01-01",
			EndDate:      "2019-06-30",
			Description:  "Collaborated with internal teams to create and train ChatBot applications using Kata.AI tools. Participated in ChatBot training for the development of the Jejakin project and developed RestAPI using the Flask framework (Python) for the final project. The project concluded with the successful development of a chatbot for hotel booking.",
			IconName:     "LuBriefcase",
			DisplayOrder: 5,
		},
	}

	for _, exp := range experiences {
		_, err = tx.ExecContext(ctx, `
INSERT INTO experiences (id, job_title, company_name, company_logo_url, location, start_date, end_date, description, icon_name, display_order, created_at, updated_at)
VALUES (?, ?, ?, '', ?, ?, ?, ?, ?, ?, ?, ?)`,
			exp.ID, exp.JobTitle, exp.CompanyName, exp.Location, exp.StartDate, exp.EndDate, exp.Description, exp.IconName, exp.DisplayOrder, now, now,
		)
		if err != nil {
			return err
		}
	}

	certificates := []models.Certificate{
		{
			ID:                  "6a67d504-90ae-4a35-b197-536713ce1526",
			Title:               "DICAS & DICSS 9th Djuanda",
			IssuingOrganization: "The 9th Djuanda International Conference on Applied Sciences (DICAS) & The 9th Djuanda International Conference on Social Sciences (DICSS)",
			IssueDate:           "2025-12-22",
			CertificateFileURL:  "/static/files/certificates/dicas-dicss-9th.pdf",
		},
		{
			ID:                  "5e8a4673-3420-44c6-862a-0cf9043499ab",
			Title:               "Full Stack Developer",
			IssuingOrganization: "Dinas Perhubungan Kota Bogor — Full Stack Developer Certificate",
			IssueDate:           "2024-07-22",
			CertificateFileURL:  "/static/files/certificates/magang-dishub.pdf",
		},
		{
			ID:                  "91ab8c9f-74c9-4763-90d8-515250a7f050",
			Title:               "Google Data Analytics",
			IssuingOrganization: "Google Data Analytics Professional Certificate",
			IssueDate:           "2024-01-29",
			CertificateFileURL:  "/static/files/certificates/google-data-analytics.pdf",
		},
		{
			ID:                  "6955a167-1174-4f5c-b6bd-c6caf27be4cd",
			Title:               "Google IT Support",
			IssuingOrganization: "Google IT Support Professional Certificate",
			IssueDate:           "2023-06-13",
			CertificateFileURL:  "/static/files/certificates/google-it-support.pdf",
		},
		{
			ID:                  "c37c6c52-aef6-4280-b93c-2aa6cff93b5f",
			Title:               "Programmer Intern",
			IssuingOrganization: "PT. Nawa Darsana Teknologi — Programmer Internship Certificate",
			IssueDate:           "2019-01-21",
			CertificateFileURL:  "/static/files/certificates/magang-nawa.pdf",
		},
	}

	for _, cert := range certificates {
		_, err = tx.ExecContext(ctx, `
INSERT INTO certificates (id, title, issuing_organization, issue_date, credential_url, certificate_file_url, created_at)
VALUES (?, ?, ?, ?, '', ?, ?)`,
			cert.ID, cert.Title, cert.IssuingOrganization, cert.IssueDate, cert.CertificateFileURL, now,
		)
		if err != nil {
			return err
		}
	}

	for _, proj := range seededProjectList() {
		techBytes, _ := json.Marshal(proj.Technologies)
		hlBytes, _ := json.Marshal(proj.Highlights)
		featInt := 0
		if proj.Featured {
			featInt = 1
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO projects (id, title, description, image_url, project_link, source_code_link, technologies_json, featured, category, role, year, highlights_json, display_order, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			proj.ID, proj.Title, proj.Description, proj.ImageURL, proj.ProjectLink, proj.SourceCodeLink, string(techBytes), featInt, proj.Category, proj.Role, proj.Year, string(hlBytes), proj.DisplayOrder, now, now,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func seededProjectList() []models.Project {
	return []models.Project{
		{
			ID:           "7f49cfa3-8de7-4cd3-8db4-69492e387ec4",
			Title:        "E-LGCertified",
			Description:  "A powerful and flexible certificate generation platform designed to simplify the way you create, customize, and manage certificates. Generate dynamic, professional certificates effortlessly with greater flexibility.",
			ImageURL:     "/static/images/projects/e-lgcertified.webp",
			ProjectLink:  "https://e-lgcertified.com",
			Technologies: []string{"Laravel", "MySQL", "PHP", "React", "Tailwind CSS", "Shadcn UI"},
			Featured:     true,
			Category:     "Production",
			Role:         "Full-Stack Developer",
			Year:         "2026",
			Highlights: []string{
				"Dynamic certificate template designer with customizable layout and batch generation",
				"Full-stack Laravel & MySQL backend paired with a responsive React + Shadcn UI dashboard",
				"Public credential verification and automated PDF/image export workflow",
			},
			DisplayOrder: 1,
		},
		{
			ID:           "c6e4aafb-3502-4aad-b7d1-f371f8227efd",
			Title:        "SIMKAN",
			Description:  "Built SIMKAN, a KKL & KKN Management Workflow System for the Computer Science Faculty at Djuanda University as my thesis project.",
			ImageURL:     "/static/images/projects/simkan.webp",
			ProjectLink:  "https://simkan.unida.education",
			Technologies: []string{"Laravel", "MySQL", "PHP", "React", "Tailwind CSS", "TypeScript"},
			Featured:     false,
			Category:     "Academic",
			Role:         "Full-Stack Developer (Thesis)",
			Year:         "2026",
			Highlights: []string{
				"End-to-end KKL & KKN internship/community service workflow for Djuanda University",
				"Role-based multi-stage approvals, advisor assignment, and document submission tracking",
				"Type-safe React + TypeScript UI powered by a Laravel and MySQL REST backend",
			},
			DisplayOrder: 2,
		},
		{
			ID:           "b5a9a90c-c846-422a-b8ff-74698f2e7d45",
			Title:        "Global Competency",
			Description:  "Company profile website for certification and training company, showcasing services, portfolio, and contact information. Built with Laravel and React JS with MySQL database.",
			ImageURL:     "/static/images/projects/globalcompetency.webp",
			ProjectLink:  "https://globalcompetency.id/",
			Technologies: []string{"Laravel", "MySQL", "PHP", "React", "Tailwind CSS", "Shadcn UI"},
			Featured:     false,
			Category:     "Client Work",
			Role:         "Full-Stack Developer",
			Year:         "2025",
			Highlights: []string{
				"Corporate profile and training catalog for professional certification services",
				"Dynamic admin CMS built with Laravel, MySQL, React, and Shadcn UI",
			},
			DisplayOrder: 3,
		},
		{
			ID:           "50941b6e-2dfa-4812-990f-bbbe03cd8c9d",
			Title:        "Cigi Global",
			Description:  "Successfully launched a polished, professional website that clearly communicates CIGI Global's complex value proposition.",
			ImageURL:     "/static/images/projects/cigi-global.webp",
			ProjectLink:  "https://cigiglobal.neddyap.tech/",
			Technologies: []string{"Laravel", "MySQL", "PHP", "React", "Tailwind CSS", "TypeScript"},
			Featured:     false,
			Category:     "Client Work",
			Role:         "Full-Stack Developer",
			Year:         "2025",
			Highlights: []string{
				"Structured multi-service presentation tailored for enterprise stakeholders",
				"Built with Laravel, React, TypeScript, and Tailwind CSS for responsive delivery",
			},
			DisplayOrder: 4,
		},
		{
			ID:             "3771d14e-d479-4a12-8ce4-6d817b884636",
			Title:          "Vehicle Management System",
			Description:    "Web app designed to help users manage their vehicles, including adding, updating, and deleting vehicle records. Built with Laravel and MySQL.",
			ImageURL:       "/static/images/projects/management-kendaraan.webp",
			ProjectLink:    "https://manajemen-kendaraan.neddyap.tech/",
			SourceCodeLink: "https://github.com/neddyap/management-kendaraan",
			Technologies:   []string{"Laravel", "MySQL", "PHP", "Bootstrap"},
			Featured:       false,
			Category:       "Web App",
			Role:           "Full-Stack Intern",
			Year:           "2024",
			Highlights: []string{
				"Developed for Dinas Perhubungan Kota Bogor after cleansing internal fleet datasets",
				"Fleet CRUD, maintenance scheduling, and service history logs in Laravel & MySQL",
			},
			DisplayOrder: 5,
		},
		{
			ID:           "01b7455b-eefa-4649-9de9-8eb84ab1b28c",
			Title:        "Website Desa",
			Description:  "Digital platform for Lemah Duhur Village integrating online public services, a citizen information center, and a promotional gallery for local potential.",
			ImageURL:     "/static/images/projects/website-desa.webp",
			ProjectLink:  "https://desalemahduhur.neddyap.tech/",
			Technologies: []string{"Laravel", "MySQL", "PHP", "React", "Tailwind CSS", "TypeScript"},
			Featured:     false,
			Category:     "Client Work",
			Role:         "Full-Stack Developer",
			Year:         "2025",
			Highlights: []string{
				"Citizen information portal and streamlined online public service requests",
				"Promotional gallery showcasing local enterprises and village potential",
			},
			DisplayOrder: 6,
		},
		{
			ID:             "694d1795-eadc-4c03-993c-493fc429cbfc",
			Title:          "Refi",
			Description:    "Flutter-based mobile application designed to help users discover, track, and manage movies. Leverages The Movie Database (TMDB) API for a rich browsing experience.",
			ImageURL:       "/static/images/projects/refi.webp",
			SourceCodeLink: "https://github.com/neddyap/refi",
			Technologies:   []string{"Flutter", "Dart", "REST API"},
			Featured:       false,
			Category:       "Mobile",
			Role:           "Mobile Developer",
			Year:           "2024",
			Highlights: []string{
				"Cross-platform Flutter app powered by The Movie Database (TMDB) REST API",
				"Movie discovery, search, ratings, and personal watchlist management",
			},
			DisplayOrder: 7,
		},
	}
}
