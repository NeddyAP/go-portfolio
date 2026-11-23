(function () {
  "use strict";

  // Navbar scrolled state
  var navbar = document.getElementById("navbar");
  if (navbar) {
    var updateNavbar = function () {
      navbar.classList.toggle("is-scrolled", window.scrollY > 40);
    };
    window.addEventListener("scroll", updateNavbar, { passive: true });
    updateNavbar();
  }

  // Theme toggle (resolves actual current class so system mode toggles cleanly on first click)
  var themeBtn = document.getElementById("theme-toggle");
  if (themeBtn) {
    themeBtn.addEventListener("click", function () {
      var nextDark = !document.documentElement.classList.contains("dark");
      document.documentElement.classList.toggle("dark", nextDark);
      try {
        localStorage.setItem("theme", nextDark ? "dark" : "light");
      } catch (_) {}
    });
  }

  // Mobile navigation drawer
  var mobileOpenBtn = document.getElementById("mobile-menu-open");
  var mobileCloseBtn = document.getElementById("mobile-menu-close");
  var mobileDrawer = document.getElementById("mobile-drawer");
  var mobileBackdrop = document.getElementById("mobile-drawer-backdrop");

  function setMobileDrawer(open) {
    if (!mobileDrawer || !mobileBackdrop) return;
    mobileDrawer.hidden = !open;
    mobileBackdrop.hidden = !open;
    if (mobileOpenBtn) mobileOpenBtn.setAttribute("aria-expanded", open ? "true" : "false");
  }

  if (mobileOpenBtn) mobileOpenBtn.addEventListener("click", function () { setMobileDrawer(true); });
  if (mobileCloseBtn) mobileCloseBtn.addEventListener("click", function () { setMobileDrawer(false); });
  if (mobileBackdrop) mobileBackdrop.addEventListener("click", function () { setMobileDrawer(false); });

  if (mobileDrawer) {
    mobileDrawer.querySelectorAll(".mobile-nav-link").forEach(function (link) {
      link.addEventListener("click", function () { setMobileDrawer(false); });
    });
  }

  // Copy email helper
  var copyEmailBtn = document.getElementById("copy-email-btn");
  if (copyEmailBtn && navigator.clipboard) {
    copyEmailBtn.addEventListener("click", function () {
      var email = copyEmailBtn.getAttribute("data-email");
      if (!email) return;
      navigator.clipboard.writeText(email).then(function () {
        var orig = copyEmailBtn.textContent;
        copyEmailBtn.textContent = "Copied!";
        setTimeout(function () { copyEmailBtn.textContent = orig; }, 2000);
      }).catch(function () {});
    });
  }

  // Certificates Carousel
  var carousel = document.getElementById("cert-carousel");
  if (carousel) {
    var track = document.getElementById("cert-carousel-track");
    var total = parseInt(carousel.getAttribute("data-total") || "0", 10);
    var prevBtn = document.getElementById("cert-prev");
    var nextBtn = document.getElementById("cert-next");
    var dots = Array.prototype.slice.call(carousel.querySelectorAll(".carousel-dot"));
    var current = 0;
    var touchStartX = null;

    function goToSlide(idx) {
      if (total <= 0 || !track) return;
      current = ((idx % total) + total) % total;
      track.style.transform = "translateX(-" + (current * 100) + "%)";
      dots.forEach(function (d, i) {
        d.classList.toggle("is-active", i === current);
      });
    }

    if (prevBtn) prevBtn.addEventListener("click", function () { goToSlide(current - 1); });
    if (nextBtn) nextBtn.addEventListener("click", function () { goToSlide(current + 1); });
    dots.forEach(function (dot) {
      dot.addEventListener("click", function () {
        goToSlide(parseInt(dot.getAttribute("data-slide") || "0", 10));
      });
    });

    carousel.addEventListener("touchstart", function (e) {
      if (e.touches && e.touches.length === 1) touchStartX = e.touches[0].clientX;
    }, { passive: true });
    carousel.addEventListener("touchend", function (e) {
      if (touchStartX === null || !e.changedTouches || e.changedTouches.length !== 1) return;
      var delta = e.changedTouches[0].clientX - touchStartX;
      touchStartX = null;
      if (Math.abs(delta) > 45) {
        goToSlide(delta < 0 ? current + 1 : current - 1);
      }
    }, { passive: true });
  }

  // Lazy PDF Preview + Fullscreen Viewer
  var fsModal = document.getElementById("pdf-fullscreen-modal");
  var fsBody = document.getElementById("pdf-fullscreen-body");
  var fsOpen = document.getElementById("pdf-fullscreen-open");
  var fsClose = document.getElementById("pdf-fullscreen-close");

  function closeFullscreenPDF() {
    if (!fsModal || !fsBody) return;
    fsModal.hidden = true;
    fsBody.innerHTML = "";
  }
  if (fsClose) fsClose.addEventListener("click", closeFullscreenPDF);

  document.querySelectorAll(".pdf-lazy-box").forEach(function (box) {
    var pdfUrl = box.getAttribute("data-pdf-url") || "";
    var placeholder = box.querySelector(".pdf-placeholder");
    var loaded = box.querySelector(".pdf-loaded");
    var frameWrap = box.querySelector(".pdf-frame-wrap");
    var loadBtn = box.querySelector(".js-pdf-load");
    var hideBtn = box.querySelector(".js-pdf-hide");
    var fullBtn = box.querySelector(".js-pdf-fullscreen");

    if (loadBtn && placeholder && loaded && frameWrap) {
      loadBtn.addEventListener("click", function () {
        if (!frameWrap.querySelector("iframe")) {
          var iframe = document.createElement("iframe");
          iframe.src = pdfUrl + "#view=FitH";
          iframe.title = "Certificate PDF preview";
          iframe.className = "pdf-iframe";
          frameWrap.appendChild(iframe);
        }
        placeholder.hidden = true;
        loaded.hidden = false;
      });
    }
    if (hideBtn && placeholder && loaded && frameWrap) {
      hideBtn.addEventListener("click", function () {
        loaded.hidden = true;
        placeholder.hidden = false;
        frameWrap.innerHTML = "";
      });
    }
    if (fullBtn && fsModal && fsBody && fsOpen) {
      fullBtn.addEventListener("click", function () {
        fsOpen.href = pdfUrl;
        fsBody.innerHTML = "";
        var iframe = document.createElement("iframe");
        iframe.src = pdfUrl + "#view=FitH";
        iframe.title = "Certificate PDF fullscreen preview";
        iframe.className = "pdf-iframe";
        fsBody.appendChild(iframe);
        fsModal.hidden = false;
      });
    }
  });

  function isSafeHref(raw) {
    if (!raw || typeof raw !== "string") return false;
    var trimmed = raw.trim();
    if (!trimmed) return false;
    if (trimmed.charAt(0) === "/" && trimmed.charAt(1) !== "/") return true;
    try {
      var parsed = new URL(trimmed, window.location.origin);
      return parsed.protocol === "http:" || parsed.protocol === "https:";
    } catch (_) {
      return false;
    }
  }

  function displayProjectHost(proj) {
    var target = (proj && (proj.project_link || proj.source_code_link)) ? String(proj.project_link || proj.source_code_link).trim() : "";
    if (!target) return "showcase";
    try {
      var parsed = new URL(target);
      var host = parsed.hostname.toLowerCase().replace(/^www\./, "");
      if (host === "github.com") {
        var path = parsed.pathname.replace(/^\/+|\/+$/g, "");
        if (path) return "github.com/" + path;
      }
      return host || target;
    } catch (_) {
      return target;
    }
  }

  // Portfolio Filter Pills
  var filterBar = document.getElementById("portfolio-filters");
  var portfolioEmpty = document.getElementById("portfolio-empty");
  var featuredWrap = document.querySelector(".featured-projects");

  if (filterBar) {
    filterBar.addEventListener("click", function (e) {
      var pill = e.target.closest(".filter-pill[data-filter]");
      if (!pill) return;
      var activeFilter = pill.getAttribute("data-filter") || "all";

      filterBar.querySelectorAll(".filter-pill[data-filter]").forEach(function (btn) {
        var isMatch = btn === pill;
        btn.classList.toggle("is-active", isMatch);
        btn.setAttribute("aria-pressed", isMatch ? "true" : "false");
      });

      var visibleCount = 0;
      var visibleFeatured = 0;
      document.querySelectorAll("#portfolio .project-filterable").forEach(function (card) {
        var keys = (card.getAttribute("data-filter-keys") || "").trim().split(/\s+/).filter(Boolean);
        var matches = activeFilter === "all" || keys.indexOf(activeFilter) !== -1;
        card.hidden = !matches;
        if (matches) {
          visibleCount++;
          if (card.classList.contains("project-featured-card")) visibleFeatured++;
          if (card.classList.contains("reveal-pending")) {
            card.classList.remove("reveal-pending");
            card.classList.add("is-visible");
          }
        }
      });

      if (featuredWrap) featuredWrap.hidden = visibleFeatured === 0;
      if (portfolioEmpty) portfolioEmpty.hidden = visibleCount > 0;
    });
  }

  // Portfolio Case-Study Modal & Image Lightbox
  var projectModal = document.getElementById("project-detail-modal");
  var projectLightbox = document.getElementById("project-lightbox");
  var lightboxImg = document.getElementById("project-lightbox-img");
  var lightboxClose = document.getElementById("project-lightbox-close");
  var modalClose = document.getElementById("project-modal-close");
  var modalBadges = document.getElementById("project-modal-badges");
  var modalMedia = document.getElementById("project-modal-media");
  var modalHost = document.getElementById("project-modal-host");
  var modalZoomBtn = document.getElementById("project-modal-zoom-btn");
  var modalImg = document.getElementById("project-modal-img");
  var modalTitle = document.getElementById("project-modal-title");
  var modalMeta = document.getElementById("project-modal-meta");
  var modalDesc = document.getElementById("project-modal-desc");
  var modalHighlightsWrap = document.getElementById("project-modal-highlights-wrap");
  var modalHighlights = document.getElementById("project-modal-highlights");
  var modalTagsWrap = document.getElementById("project-modal-tags-wrap");
  var modalTags = document.getElementById("project-modal-tags");
  var modalLinks = document.getElementById("project-modal-links");
  var modalPrev = document.getElementById("project-modal-prev");
  var modalNext = document.getElementById("project-modal-next");
  var visibleModalCards = [];
  var currentModalIndex = 0;

  function makeSvgIcon(pathD, extraAttrs) {
    var svgNS = "http://www.w3.org/2000/svg";
    var svg = document.createElementNS(svgNS, "svg");
    svg.setAttribute("viewBox", "0 0 24 24");
    svg.setAttribute("fill", "none");
    svg.setAttribute("stroke", "currentColor");
    svg.setAttribute("stroke-width", "2");
    svg.setAttribute("stroke-linecap", "round");
    svg.setAttribute("stroke-linejoin", "round");
    svg.setAttribute("class", "icon-sm");
    svg.setAttribute("aria-hidden", "true");
    pathD.forEach(function (d) {
      var p = document.createElementNS(svgNS, "path");
      p.setAttribute("d", d);
      svg.appendChild(p);
    });
    if (extraAttrs && extraAttrs.circle) {
      var c = document.createElementNS(svgNS, "circle");
      Object.keys(extraAttrs.circle).forEach(function (k) { c.setAttribute(k, extraAttrs.circle[k]); });
      svg.appendChild(c);
    }
    return svg;
  }

  function renderBadge(cls, text, withLiveDot) {
    var span = document.createElement("span");
    span.className = "status-badge " + cls;
    if (withLiveDot) {
      var dot = document.createElement("span");
      dot.className = "live-dot";
      dot.setAttribute("aria-hidden", "true");
      span.appendChild(dot);
    }
    span.appendChild(document.createTextNode(text));
    return span;
  }

  function closeLightbox() {
    if (!projectLightbox) return;
    projectLightbox.hidden = true;
    if (lightboxImg) {
      lightboxImg.removeAttribute("src");
      lightboxImg.alt = "";
    }
  }

  function renderProjectModalAt(index) {
    if (!projectModal || !visibleModalCards.length) return;
    currentModalIndex = (index + visibleModalCards.length) % visibleModalCards.length;
    var card = visibleModalCards[currentModalIndex];
    var raw = card.getAttribute("data-project");
    if (!raw) return;
    var proj;
    try {
      proj = JSON.parse(raw);
    } catch (_) {
      return;
    }

    if (modalBadges) {
      modalBadges.innerHTML = "";
      if (proj.featured) modalBadges.appendChild(renderBadge("badge-featured", "Featured", false));
      if (proj.project_link) modalBadges.appendChild(renderBadge("badge-live", "Live", true));
      if (proj.source_code_link) modalBadges.appendChild(renderBadge("badge-oss", "Open Source", false));
      if (proj.category) modalBadges.appendChild(renderBadge("badge-cat", proj.category, false));
    }

    if (modalMedia && modalImg && modalHost) {
      var frame = modalMedia.querySelector(".browser-frame");
      if (frame) frame.classList.toggle("is-mobile", proj.category === "Mobile");
      if (isSafeHref(proj.image_url)) {
        modalImg.src = proj.image_url;
        modalImg.alt = proj.title || "Project screenshot";
        modalHost.textContent = displayProjectHost(proj);
        modalMedia.hidden = false;
      } else {
        modalImg.removeAttribute("src");
        modalImg.alt = "";
        modalMedia.hidden = true;
      }
    }

    if (modalTitle) modalTitle.textContent = proj.title || "";
    if (modalMeta) {
      modalMeta.innerHTML = "";
      if (proj.role) {
        var rSpan = document.createElement("span");
        rSpan.textContent = proj.role;
        modalMeta.appendChild(rSpan);
      }
      if (proj.role && proj.year) {
        var sep = document.createElement("span");
        sep.setAttribute("aria-hidden", "true");
        sep.textContent = "·";
        modalMeta.appendChild(sep);
      }
      if (proj.year) {
        var ySpan = document.createElement("span");
        ySpan.textContent = proj.year;
        modalMeta.appendChild(ySpan);
      }
      modalMeta.hidden = !proj.role && !proj.year;
    }
    if (modalDesc) modalDesc.textContent = proj.description || "";

    if (modalHighlightsWrap && modalHighlights) {
      modalHighlights.innerHTML = "";
      var hs = Array.isArray(proj.highlights) ? proj.highlights : [];
      hs.forEach(function (item) {
        if (!item) return;
        var li = document.createElement("li");
        li.appendChild(makeSvgIcon(["M20 6 9 17l-5-5"]));
        var txt = document.createElement("span");
        txt.textContent = item;
        li.appendChild(txt);
        modalHighlights.appendChild(li);
      });
      modalHighlightsWrap.hidden = hs.length === 0;
    }

    if (modalTagsWrap && modalTags) {
      modalTags.innerHTML = "";
      var techs = Array.isArray(proj.technologies) ? proj.technologies : [];
      techs.forEach(function (t) {
        if (!t) return;
        var pill = document.createElement("span");
        pill.className = "tech-pill";
        pill.textContent = t;
        modalTags.appendChild(pill);
      });
      modalTagsWrap.hidden = techs.length === 0;
    }

    if (modalLinks) {
      modalLinks.innerHTML = "";
      if (isSafeHref(proj.project_link)) {
        var demoA = document.createElement("a");
        demoA.href = proj.project_link;
        demoA.target = "_blank";
        demoA.rel = "noopener noreferrer";
        demoA.className = "btn btn-primary btn-sm";
        demoA.appendChild(makeSvgIcon(["M15 3h6v6", "M10 14 21 3", "M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"]));
        demoA.appendChild(document.createTextNode("Demo"));
        modalLinks.appendChild(demoA);
      }
      if (isSafeHref(proj.source_code_link)) {
        var srcA = document.createElement("a");
        srcA.href = proj.source_code_link;
        srcA.target = "_blank";
        srcA.rel = "noopener noreferrer";
        srcA.className = "btn btn-outline btn-sm";
        srcA.appendChild(makeSvgIcon([
          "M15 22v-4a4.8 4.8 0 0 0-1-3.5c3 0 6-2 6-5.5.08-1.25-.27-2.48-1-3.5.28-1.15.28-2.35 0-3.5 0 0-1 0-3 1.5-2.64-.5-5.36-.5-8 0C6 2 5 2 5 2c-.3 1.15-.3 2.35 0 3.5A5.403 5.403 0 0 0 4 9c0 3.5 3 5.5 6 5.5-.39.49-.68 1.05-.85 1.65-.17.6-.22 1.23-.15 1.85v4",
          "M9 18c-4.51 2-5-2-7-2"
        ]));
        srcA.appendChild(document.createTextNode("Source"));
        modalLinks.appendChild(srcA);
      }
    }

    var multi = visibleModalCards.length > 1;
    if (modalPrev) modalPrev.disabled = !multi;
    if (modalNext) modalNext.disabled = !multi;

    var scrollBox = projectModal.querySelector(".project-modal-scroll");
    if (scrollBox) scrollBox.scrollTop = 0;
    projectModal.hidden = false;
  }

  var portfolioSection = document.getElementById("portfolio");
  if (portfolioSection && projectModal) {
    portfolioSection.addEventListener("click", function (e) {
      var trigger = e.target.closest(".js-open-project");
      if (!trigger) return;
      var card = trigger.closest(".project-filterable");
      if (!card) return;
      visibleModalCards = [].slice.call(portfolioSection.querySelectorAll(".project-filterable")).filter(function (el) {
        return !el.hidden;
      });
      var idx = visibleModalCards.indexOf(card);
      if (idx === -1) return;
      renderProjectModalAt(idx);
      if (modalClose) modalClose.focus();
    });
  }

  if (modalClose && projectModal) {
    modalClose.addEventListener("click", function () {
      closeLightbox();
      projectModal.hidden = true;
    });
  }
  if (projectModal) {
    projectModal.addEventListener("click", function (e) {
      if (e.target === projectModal) {
        closeLightbox();
        projectModal.hidden = true;
      }
    });
  }
  if (modalPrev) {
    modalPrev.addEventListener("click", function () { renderProjectModalAt(currentModalIndex - 1); });
  }
  if (modalNext) {
    modalNext.addEventListener("click", function () { renderProjectModalAt(currentModalIndex + 1); });
  }
  if (modalZoomBtn && projectLightbox && lightboxImg && modalImg) {
    modalZoomBtn.addEventListener("click", function () {
      if (!modalImg.getAttribute("src")) return;
      lightboxImg.src = modalImg.getAttribute("src");
      lightboxImg.alt = modalImg.alt || "";
      projectLightbox.hidden = false;
      if (lightboxClose) lightboxClose.focus();
    });
  }
  if (lightboxClose) lightboxClose.addEventListener("click", closeLightbox);
  if (projectLightbox) {
    projectLightbox.addEventListener("click", function (e) {
      if (e.target === projectLightbox) closeLightbox();
    });
  }

  // Admin Login Modal
  var loginTrigger = document.getElementById("admin-login-trigger");
  var loginModal = document.getElementById("login-modal");
  var loginForm = document.getElementById("login-form");
  var loginError = document.getElementById("login-error");
  var loginSubmit = document.getElementById("login-submit");

  function openLoginModal() {
    if (!loginModal) return;
    loginModal.hidden = false;
    var emailInput = document.getElementById("login-email");
    if (emailInput) emailInput.focus();
  }
  function closeModals() {
    closeLightbox();
    document.querySelectorAll(".modal-overlay").forEach(function (m) { m.hidden = true; });
  }

  if (loginTrigger) loginTrigger.addEventListener("click", openLoginModal);
  document.querySelectorAll("[data-close-modal]").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var id = btn.getAttribute("data-close-modal");
      var el = id ? document.getElementById(id) : null;
      if (el) el.hidden = true;
    });
  });

  if (loginForm) {
    loginForm.addEventListener("submit", function (e) {
      e.preventDefault();
      if (loginError) loginError.hidden = true;
      if (loginSubmit) loginSubmit.disabled = true;

      var email = (document.getElementById("login-email") || {}).value || "";
      var password = (document.getElementById("login-password") || {}).value || "";

      fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ email: email, password: password })
      }).then(function (res) {
        return res.json().then(function (data) { return { ok: res.ok, data: data }; });
      }).then(function (result) {
        if (!result.ok) {
          throw new Error(result.data.error || "Login failed");
        }
        window.location.reload();
      }).catch(function (err) {
        if (loginError) {
          loginError.textContent = err.message || "Login failed";
          loginError.hidden = false;
        }
      }).finally(function () {
        if (loginSubmit) loginSubmit.disabled = false;
      });
    });
  }

  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape") {
      if (projectLightbox && !projectLightbox.hidden) {
        closeLightbox();
        return;
      }
      setMobileDrawer(false);
      closeFullscreenPDF();
      closeModals();
      return;
    }
    if (projectModal && !projectModal.hidden && (!projectLightbox || projectLightbox.hidden)) {
      if (e.key === "ArrowLeft") {
        e.preventDefault();
        renderProjectModalAt(currentModalIndex - 1);
      } else if (e.key === "ArrowRight") {
        e.preventDefault();
        renderProjectModalAt(currentModalIndex + 1);
      }
    }
  });

  // Subtle progressive scroll reveal
  var reducedMotion = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  if (!reducedMotion && "IntersectionObserver" in window) {
    var reveals = document.querySelectorAll(".reveal");
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          entry.target.classList.remove("reveal-pending");
          entry.target.classList.add("is-visible");
          io.unobserve(entry.target);
        }
      });
    }, { threshold: 0.08 });

    reveals.forEach(function (el) {
      var rect = el.getBoundingClientRect();
      if (rect.top > window.innerHeight * 0.95) {
        el.classList.add("reveal-pending");
        io.observe(el);
      }
    });
  }
})();
