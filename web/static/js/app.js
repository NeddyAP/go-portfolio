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
      setMobileDrawer(false);
      closeFullscreenPDF();
      closeModals();
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
