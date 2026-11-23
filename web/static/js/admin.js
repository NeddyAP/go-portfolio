(function () {
  "use strict";

  function getCSRFToken() {
    var parts = document.cookie.split("; ");
    for (var i = 0; i < parts.length; i++) {
      var idx = parts[i].indexOf("=");
      if (idx > 0 && parts[i].slice(0, idx) === "portfolio_csrf") {
        return decodeURIComponent(parts[i].slice(idx + 1));
      }
    }
    return "";
  }

  function showToast(message, isError) {
    var container = document.getElementById("toast-container");
    if (!container) return;
    var el = document.createElement("div");
    el.className = "toast" + (isError ? " toast-error" : "");
    el.textContent = message;
    container.appendChild(el);
    setTimeout(function () {
      el.remove();
    }, 4000);
  }

  function apiRequest(url, method, bodyObj) {
    var opts = {
      method: method,
      credentials: "include",
      headers: {
        "X-CSRF-Token": getCSRFToken()
      }
    };
    if (bodyObj !== undefined) {
      opts.headers["Content-Type"] = "application/json";
      opts.body = JSON.stringify(bodyObj);
    }
    return fetch(url, opts).then(function (res) {
      return res.json().catch(function () { return {}; }).then(function (data) {
        if (!res.ok) {
          throw new Error(data.error || data.message || ("Request failed (" + res.status + ")"));
        }
        return data;
      });
    });
  }

  function uploadFile(file, type, currentUrl) {
    var fd = new FormData();
    fd.append("file", file);
    fd.append("type", type);
    if (currentUrl) fd.append("currentFileUrl", currentUrl);
    return fetch("/api/upload", {
      method: "POST",
      credentials: "include",
      headers: { "X-CSRF-Token": getCSRFToken() },
      body: fd
    }).then(function (res) {
      return res.json().then(function (data) {
        if (!res.ok) throw new Error(data.error || "Upload failed");
        return data.url;
      });
    });
  }

  var backdrop = document.getElementById("admin-sheet-backdrop");
  function openSheet(id) {
    closeSheets();
    var sheet = document.getElementById(id);
    if (!sheet || !backdrop) return;
    backdrop.hidden = false;
    sheet.hidden = false;
  }
  function closeSheets() {
    if (backdrop) backdrop.hidden = true;
    document.querySelectorAll(".admin-sheet").forEach(function (s) { s.hidden = true; });
  }
  if (backdrop) backdrop.addEventListener("click", closeSheets);
  document.querySelectorAll(".js-close-sheet").forEach(function (btn) {
    btn.addEventListener("click", closeSheets);
  });
  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape") closeSheets();
  });

  // Logout
  function handleLogout() {
    apiRequest("/api/auth/logout", "POST").then(function () {
      window.location.reload();
    }).catch(function (err) {
      showToast(err.message, true);
    });
  }
  var logoutBtn = document.getElementById("admin-logout-btn");
  var mobileLogoutBtn = document.getElementById("mobile-admin-logout-btn");
  if (logoutBtn) logoutBtn.addEventListener("click", handleLogout);
  if (mobileLogoutBtn) mobileLogoutBtn.addEventListener("click", handleLogout);

  // Bootstrap About Me data
  var aboutBootstrapEl = document.getElementById("admin-bootstrap");
  var aboutState = aboutBootstrapEl ? JSON.parse(aboutBootstrapEl.textContent || "{}") : {};
  var hobbiesState = Array.isArray(aboutState.hobbies) ? aboutState.hobbies.slice() : [];
  var skillsetState = Array.isArray(aboutState.skillset) ? aboutState.skillset.slice() : [];
  var toolsState = Array.isArray(aboutState.tools) ? aboutState.tools.slice() : [];

  function renderIconItemList(listEl, items, allowReorder) {
    if (!listEl) return;
    listEl.innerHTML = "";
    items.forEach(function (item, idx) {
      var li = document.createElement("li");
      li.className = "admin-item-row";

      var labelWrap = document.createElement("div");
      var strong = document.createElement("strong");
      strong.textContent = item.name;
      labelWrap.appendChild(strong);
      if (item.icon_name) {
        var sub = document.createElement("span");
        sub.className = "admin-item-sub";
        sub.textContent = "(" + item.icon_name + ")";
        labelWrap.appendChild(sub);
      }

      var actions = document.createElement("div");
      actions.className = "admin-item-actions";

      if (allowReorder) {
        var upBtn = document.createElement("button");
        upBtn.type = "button";
        upBtn.className = "btn btn-ghost btn-sm";
        upBtn.textContent = "↑";
        upBtn.disabled = idx === 0;
        upBtn.addEventListener("click", function () {
          var tmp = items[idx - 1];
          items[idx - 1] = items[idx];
          items[idx] = tmp;
          items.forEach(function (it, i) { it.order = i; });
          renderIconItemList(listEl, items, allowReorder);
        });

        var downBtn = document.createElement("button");
        downBtn.type = "button";
        downBtn.className = "btn btn-ghost btn-sm";
        downBtn.textContent = "↓";
        downBtn.disabled = idx === items.length - 1;
        downBtn.addEventListener("click", function () {
          var tmp = items[idx + 1];
          items[idx + 1] = items[idx];
          items[idx] = tmp;
          items.forEach(function (it, i) { it.order = i; });
          renderIconItemList(listEl, items, allowReorder);
        });
        actions.appendChild(upBtn);
        actions.appendChild(downBtn);
      }

      var delBtn = document.createElement("button");
      delBtn.type = "button";
      delBtn.className = "btn btn-ghost btn-sm text-danger";
      delBtn.textContent = "✕";
      delBtn.addEventListener("click", function () {
        items.splice(idx, 1);
        items.forEach(function (it, i) { it.order = i; });
        renderIconItemList(listEl, items, allowReorder);
      });
      actions.appendChild(delBtn);

      li.appendChild(labelWrap);
      li.appendChild(actions);
      listEl.appendChild(li);
    });
  }

  // About Me drawer
  var editAboutBtn = document.getElementById("edit-about-btn");
  var hobbiesListEl = document.getElementById("hobbies-edit-list");
  if (editAboutBtn) {
    editAboutBtn.addEventListener("click", function () {
      document.getElementById("about-full-name").value = aboutState.full_name || "";
      document.getElementById("about-headline").value = aboutState.headline || "";
      document.getElementById("about-bio").value = aboutState.bio || "";
      document.getElementById("about-email").value = aboutState.contact_email || "";
      document.getElementById("about-github").value = aboutState.github_url || "";
      document.getElementById("about-linkedin").value = aboutState.linkedin_url || "";
      document.getElementById("about-profile-url").value = aboutState.profile_image_url || "";
      document.getElementById("about-resume-url").value = aboutState.resume_url || "";
      document.getElementById("about-quote-text").value = (aboutState.quote && aboutState.quote.text) || "";
      document.getElementById("about-quote-author").value = (aboutState.quote && aboutState.quote.author) || "";
      renderIconItemList(hobbiesListEl, hobbiesState, false);
      openSheet("sheet-about");
    });
  }

  var hobbyAddBtn = document.getElementById("hobby-add-btn");
  if (hobbyAddBtn) {
    hobbyAddBtn.addEventListener("click", function () {
      var nameEl = document.getElementById("hobby-new-name");
      var iconEl = document.getElementById("hobby-new-icon");
      var name = nameEl.value.trim();
      if (!name) return;
      hobbiesState.push({ name: name, icon_name: iconEl.value.trim(), order: hobbiesState.length });
      nameEl.value = "";
      iconEl.value = "";
      renderIconItemList(hobbiesListEl, hobbiesState, false);
    });
  }

  var profileFileEl = document.getElementById("about-profile-file");
  if (profileFileEl) {
    profileFileEl.addEventListener("change", function () {
      var file = profileFileEl.files && profileFileEl.files[0];
      if (!file) return;
      var urlEl = document.getElementById("about-profile-url");
      uploadFile(file, "profile", urlEl.value).then(function (url) {
        urlEl.value = url;
        showToast("Profile image uploaded. Save changes to apply.");
      }).catch(function (err) {
        showToast(err.message, true);
      });
    });
  }

  var resumeFileEl = document.getElementById("about-resume-file");
  if (resumeFileEl) {
    resumeFileEl.addEventListener("change", function () {
      var file = resumeFileEl.files && resumeFileEl.files[0];
      if (!file) return;
      var urlEl = document.getElementById("about-resume-url");
      uploadFile(file, "resume", urlEl.value).then(function (url) {
        urlEl.value = url;
        showToast("Resume PDF uploaded. Save changes to apply.");
      }).catch(function (err) {
        showToast(err.message, true);
      });
    });
  }

  var formAbout = document.getElementById("form-about");
  if (formAbout) {
    formAbout.addEventListener("submit", function (e) {
      e.preventDefault();
      var qt = document.getElementById("about-quote-text").value.trim();
      var qa = document.getElementById("about-quote-author").value.trim();
      var payload = {
        full_name: document.getElementById("about-full-name").value.trim(),
        headline: document.getElementById("about-headline").value.trim(),
        bio: document.getElementById("about-bio").value.trim(),
        contact_email: document.getElementById("about-email").value.trim(),
        github_url: document.getElementById("about-github").value.trim(),
        linkedin_url: document.getElementById("about-linkedin").value.trim(),
        profile_image_url: document.getElementById("about-profile-url").value.trim(),
        resume_url: document.getElementById("about-resume-url").value.trim(),
        hobbies: hobbiesState,
        clear_quote: qt === "" && qa === "",
        quote: (qt || qa) ? { text: qt, author: qa } : null
      };
      apiRequest("/api/about-me", "PUT", payload).then(function () {
        window.location.reload();
      }).catch(function (err) {
        showToast(err.message, true);
      });
    });
  }

  // Skillset drawer
  var skillsetListEl = document.getElementById("skillset-edit-list");
  var editSkillsetBtn = document.getElementById("edit-skillset-btn");
  if (editSkillsetBtn) {
    editSkillsetBtn.addEventListener("click", function () {
      renderIconItemList(skillsetListEl, skillsetState, true);
      openSheet("sheet-skillset");
    });
  }
  var skillAddBtn = document.getElementById("skill-add-btn");
  if (skillAddBtn) {
    skillAddBtn.addEventListener("click", function () {
      var nameEl = document.getElementById("skill-new-name");
      var iconEl = document.getElementById("skill-new-icon");
      var name = nameEl.value.trim();
      if (!name) return;
      skillsetState.push({ name: name, icon_name: iconEl.value.trim(), order: skillsetState.length });
      nameEl.value = "";
      iconEl.value = "";
      renderIconItemList(skillsetListEl, skillsetState, true);
    });
  }
  var skillSaveBtn = document.getElementById("skillset-save-btn");
  if (skillSaveBtn) {
    skillSaveBtn.addEventListener("click", function () {
      apiRequest("/api/about-me", "PUT", { skillset: skillsetState }).then(function () {
        window.location.reload();
      }).catch(function (err) {
        showToast(err.message, true);
      });
    });
  }

  // Tools drawer
  var toolsListEl = document.getElementById("tools-edit-list");
  var editToolsBtn = document.getElementById("edit-tools-btn");
  if (editToolsBtn) {
    editToolsBtn.addEventListener("click", function () {
      renderIconItemList(toolsListEl, toolsState, true);
      openSheet("sheet-tools");
    });
  }
  var toolAddBtn = document.getElementById("tool-add-btn");
  if (toolAddBtn) {
    toolAddBtn.addEventListener("click", function () {
      var nameEl = document.getElementById("tool-new-name");
      var iconEl = document.getElementById("tool-new-icon");
      var name = nameEl.value.trim();
      if (!name) return;
      toolsState.push({ name: name, icon_name: iconEl.value.trim(), order: toolsState.length });
      nameEl.value = "";
      iconEl.value = "";
      renderIconItemList(toolsListEl, toolsState, true);
    });
  }
  var toolsSaveBtn = document.getElementById("tools-save-btn");
  if (toolsSaveBtn) {
    toolsSaveBtn.addEventListener("click", function () {
      apiRequest("/api/about-me", "PUT", { tools: toolsState }).then(function () {
        window.location.reload();
      }).catch(function (err) {
        showToast(err.message, true);
      });
    });
  }

  // Experience drawer
  function openExpSheet(exp) {
    document.getElementById("sheet-exp-title").textContent = exp ? "Edit Experience" : "Add New Experience";
    document.getElementById("exp-id").value = exp ? exp.id : "";
    document.getElementById("exp-job-title").value = exp ? exp.job_title : "";
    document.getElementById("exp-company").value = exp ? exp.company_name : "";
    document.getElementById("exp-location").value = exp ? (exp.location || "") : "";
    document.getElementById("exp-start-date").value = exp ? (exp.start_date || "") : "";
    document.getElementById("exp-end-date").value = exp ? (exp.end_date || "") : "";
    document.getElementById("exp-description").value = exp ? (exp.description || "") : "";
    document.getElementById("exp-icon").value = exp ? (exp.icon_name || "") : "LuBriefcase";
    document.getElementById("exp-order").value = exp ? String(exp.display_order || 0) : "0";
    openSheet("sheet-experience");
  }
  var addExpBtn = document.getElementById("add-experience-btn");
  if (addExpBtn) addExpBtn.addEventListener("click", function () { openExpSheet(null); });
  document.querySelectorAll(".js-edit-exp").forEach(function (btn) {
    btn.addEventListener("click", function () {
      openExpSheet(JSON.parse(btn.getAttribute("data-item") || "{}"));
    });
  });
  var formExp = document.getElementById("form-experience");
  if (formExp) {
    formExp.addEventListener("submit", function (e) {
      e.preventDefault();
      var id = document.getElementById("exp-id").value;
      var payload = {
        job_title: document.getElementById("exp-job-title").value.trim(),
        company_name: document.getElementById("exp-company").value.trim(),
        location: document.getElementById("exp-location").value.trim(),
        start_date: document.getElementById("exp-start-date").value.trim(),
        end_date: document.getElementById("exp-end-date").value.trim(),
        description: document.getElementById("exp-description").value.trim(),
        icon_name: document.getElementById("exp-icon").value.trim(),
        display_order: parseInt(document.getElementById("exp-order").value || "0", 10) || 0
      };
      var url = id ? "/api/experiences/" + encodeURIComponent(id) : "/api/experiences";
      apiRequest(url, id ? "PUT" : "POST", payload).then(function () {
        window.location.reload();
      }).catch(function (err) {
        showToast(err.message, true);
      });
    });
  }

  // Certificate drawer
  function openCertSheet(cert) {
    document.getElementById("sheet-cert-title").textContent = cert ? "Edit Certificate" : "Add New Certificate";
    document.getElementById("cert-id").value = cert ? cert.id : "";
    document.getElementById("cert-title").value = cert ? cert.title : "";
    document.getElementById("cert-org").value = cert ? cert.issuing_organization : "";
    document.getElementById("cert-date").value = cert ? (cert.issue_date || "") : "";
    document.getElementById("cert-credential-url").value = cert ? (cert.credential_url || "") : "";
    document.getElementById("cert-file-url").value = cert ? (cert.certificate_file_url || "") : "";
    openSheet("sheet-certificate");
  }
  var addCertBtn = document.getElementById("add-certificate-btn");
  if (addCertBtn) addCertBtn.addEventListener("click", function () { openCertSheet(null); });
  document.querySelectorAll(".js-edit-cert").forEach(function (btn) {
    btn.addEventListener("click", function () {
      openCertSheet(JSON.parse(btn.getAttribute("data-item") || "{}"));
    });
  });
  var certFileEl = document.getElementById("cert-file");
  if (certFileEl) {
    certFileEl.addEventListener("change", function () {
      var file = certFileEl.files && certFileEl.files[0];
      if (!file) return;
      var urlEl = document.getElementById("cert-file-url");
      uploadFile(file, "certificate", urlEl.value).then(function (url) {
        urlEl.value = url;
        showToast("Certificate PDF uploaded. Save to persist.");
      }).catch(function (err) {
        showToast(err.message, true);
      });
    });
  }
  var formCert = document.getElementById("form-certificate");
  if (formCert) {
    formCert.addEventListener("submit", function (e) {
      e.preventDefault();
      var id = document.getElementById("cert-id").value;
      var payload = {
        title: document.getElementById("cert-title").value.trim(),
        issuing_organization: document.getElementById("cert-org").value.trim(),
        issue_date: document.getElementById("cert-date").value.trim(),
        credential_url: document.getElementById("cert-credential-url").value.trim(),
        certificate_file_url: document.getElementById("cert-file-url").value.trim()
      };
      var url = id ? "/api/certificates/" + encodeURIComponent(id) : "/api/certificates";
      apiRequest(url, id ? "PUT" : "POST", payload).then(function () {
        window.location.reload();
      }).catch(function (err) {
        showToast(err.message, true);
      });
    });
  }

  // Project drawer
  function openProjectSheet(proj) {
    document.getElementById("sheet-proj-title").textContent = proj ? "Edit Project" : "Add New Project";
    document.getElementById("proj-id").value = proj ? proj.id : "";
    document.getElementById("proj-title").value = proj ? proj.title : "";
    document.getElementById("proj-featured").checked = Boolean(proj && proj.featured);
    document.getElementById("proj-category").value = proj ? (proj.category || "") : "";
    document.getElementById("proj-role").value = proj ? (proj.role || "") : "";
    document.getElementById("proj-year").value = proj ? (proj.year || "") : "";
    document.getElementById("proj-description").value = proj ? (proj.description || "") : "";
    document.getElementById("proj-highlights").value = proj && Array.isArray(proj.highlights) ? proj.highlights.join("\n") : "";
    document.getElementById("proj-image-url").value = proj ? (proj.image_url || "") : "";
    document.getElementById("proj-link").value = proj ? (proj.project_link || "") : "";
    document.getElementById("proj-source-link").value = proj ? (proj.source_code_link || "") : "";
    document.getElementById("proj-tech").value = proj && Array.isArray(proj.technologies) ? proj.technologies.join(", ") : "";
    document.getElementById("proj-order").value = proj ? String(proj.display_order || 0) : "0";
    openSheet("sheet-project");
  }
  var addProjBtn = document.getElementById("add-project-btn");
  if (addProjBtn) addProjBtn.addEventListener("click", function () { openProjectSheet(null); });
  document.querySelectorAll(".js-edit-project").forEach(function (btn) {
    btn.addEventListener("click", function () {
      openProjectSheet(JSON.parse(btn.getAttribute("data-item") || "{}"));
    });
  });
  var projFileEl = document.getElementById("proj-image-file");
  if (projFileEl) {
    projFileEl.addEventListener("change", function () {
      var file = projFileEl.files && projFileEl.files[0];
      if (!file) return;
      var urlEl = document.getElementById("proj-image-url");
      uploadFile(file, "project", urlEl.value).then(function (url) {
        urlEl.value = url;
        showToast("Project image uploaded. Save to persist.");
      }).catch(function (err) {
        showToast(err.message, true);
      });
    });
  }
  var formProj = document.getElementById("form-project");
  if (formProj) {
    formProj.addEventListener("submit", function (e) {
      e.preventDefault();
      var id = document.getElementById("proj-id").value;
      var rawTech = document.getElementById("proj-tech").value || "";
      var techs = rawTech.split(",").map(function (t) { return t.trim(); }).filter(Boolean);
      var rawHighlights = document.getElementById("proj-highlights").value || "";
      var highlights = rawHighlights.split(/\r?\n/).map(function (h) { return h.trim(); }).filter(Boolean);
      var payload = {
        title: document.getElementById("proj-title").value.trim(),
        featured: document.getElementById("proj-featured").checked,
        category: document.getElementById("proj-category").value.trim(),
        role: document.getElementById("proj-role").value.trim(),
        year: document.getElementById("proj-year").value.trim(),
        description: document.getElementById("proj-description").value.trim(),
        highlights: highlights,
        image_url: document.getElementById("proj-image-url").value.trim(),
        project_link: document.getElementById("proj-link").value.trim(),
        source_code_link: document.getElementById("proj-source-link").value.trim(),
        technologies: techs,
        display_order: parseInt(document.getElementById("proj-order").value || "0", 10) || 0
      };
      var url = id ? "/api/projects/" + encodeURIComponent(id) : "/api/projects";
      apiRequest(url, id ? "PUT" : "POST", payload).then(function () {
        window.location.reload();
      }).catch(function (err) {
        showToast(err.message, true);
      });
    });
  }

  // Delete confirmation modal
  var deleteModal = document.getElementById("delete-modal");
  var deleteDesc = document.getElementById("delete-modal-desc");
  var deleteConfirmBtn = document.getElementById("delete-confirm-btn");
  var pendingDelete = null;

  document.querySelectorAll(".js-delete-item").forEach(function (btn) {
    btn.addEventListener("click", function () {
      pendingDelete = {
        kind: btn.getAttribute("data-kind"),
        id: btn.getAttribute("data-id"),
        label: btn.getAttribute("data-label") || "this item"
      };
      if (deleteDesc) {
        deleteDesc.textContent = 'Are you sure you want to delete "' + pendingDelete.label + '"? This action cannot be undone.';
      }
      if (deleteModal) deleteModal.hidden = false;
    });
  });

  if (deleteConfirmBtn) {
    deleteConfirmBtn.addEventListener("click", function () {
      if (!pendingDelete) return;
      var endpointMap = {
        experience: "/api/experiences/",
        certificate: "/api/certificates/",
        project: "/api/projects/"
      };
      var base = endpointMap[pendingDelete.kind];
      if (!base) return;
      apiRequest(base + encodeURIComponent(pendingDelete.id), "DELETE").then(function () {
        window.location.reload();
      }).catch(function (err) {
        showToast(err.message, true);
      });
    });
  }
})();
