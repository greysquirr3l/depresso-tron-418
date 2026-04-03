// brew.js — Client-side chaos layer for DEPRESSO-TRON 418
// All logic is intentionally over-specified for a problem that doesn't exist.

"use strict";

// ── Decaf Nuclear Protocol ──────────────────────────────────────────────────

let decafLockoutTimer = null;

function activateDecafProtocol() {
  const overlay = document.getElementById("decaf-overlay");
  if (!overlay) return;

  overlay.classList.remove("hidden");
  document.body.classList.add("decaf-active");

  let remaining = 300;
  const timerEl = document.getElementById("decaf-timer");

  clearInterval(decafLockoutTimer);
  decafLockoutTimer = setInterval(() => {
    remaining -= 1;
    if (timerEl) timerEl.textContent = String(remaining);
    if (remaining <= 0) {
      clearInterval(decafLockoutTimer);
      overlay.classList.add("hidden");
      document.body.classList.remove("decaf-active");
    }
  }, 1000);
}

// Passive early-warning: detect the forbidden word as the user types,
// not just on submit. Psychological deterrence.
function detectDecaf(value) {
  const forbidden = [
    "decaf", "de-caf", "décaf", "decaffeinated",
    "caffeine-free", "caffeine free", "no caffeine",
    "half-caf", "half caf", "nocaf",
  ];
  const lower = value.toLowerCase();
  const found = forbidden.some(f => lower.includes(f));
  const textarea = document.getElementById("beans");
  if (!textarea) return;

  if (found) {
    textarea.style.borderColor = "#e53935";
    textarea.style.boxShadow = "0 0 12px rgba(229,57,53,0.6)";
    textarea.title = "WARNING: Forbidden vocabulary detected. Submitting will activate the Decaf Nuclear Protocol.";
  } else {
    textarea.style.borderColor = "";
    textarea.style.boxShadow = "";
    textarea.title = "";
  }
}

// ── Identity Crisis Overlay ─────────────────────────────────────────────────

function activateIdentityCrisis() {
  const crtInset = document.getElementById("crt-inset");
  if (crtInset) crtInset.classList.add("crisis-expand");

  const crisis = document.getElementById("crisis-overlay");
  if (crisis) crisis.classList.remove("hidden");

  // Auto-resolve after the mood cycle (~60–120s). The server will recover.
  setTimeout(() => {
    if (crtInset) crtInset.classList.remove("crisis-expand");
    if (crisis)   crisis.classList.add("hidden");
  }, 90_000);
}

// ── Mood Watcher ────────────────────────────────────────────────────────────
// Poll the mood badge; when the server enters IDENTITY_CRISIS, trigger the
// overlay on the client side.

let lastMoodClass = "";
function watchMood() {
  const badge = document.getElementById("server-mood");
  if (!badge) return;

  const cls = badge.className;
  if (cls !== lastMoodClass) {
    lastMoodClass = cls;
    if (cls.includes("mood-crisis")) {
      activateIdentityCrisis();
    }
  }
}
setInterval(watchMood, 5_500);

// ── Gemini key form: mask/unmask toggle ──────────────────────────────────────

document.addEventListener("DOMContentLoaded", () => {
  const keyInput = document.getElementById("gemini_key");
  if (!keyInput) return;

  const toggle = document.createElement("button");
  toggle.type = "button";
  toggle.textContent = "Show";
  toggle.className = "btn-secondary";
  toggle.style.marginTop = "0.4rem";
  toggle.style.fontSize = "0.75rem";
  toggle.style.padding = "0.2em 0.6em";
  toggle.addEventListener("click", () => {
    if (keyInput.type === "password") {
      keyInput.type = "text";
      toggle.textContent = "Hide";
    } else {
      keyInput.type = "password";
      toggle.textContent = "Show";
    }
  });
  keyInput.insertAdjacentElement("afterend", toggle);
});

// ── Flee-button: tracks mouse proximity ─────────────────────────────────────
// The spec says buttons flee on hover; this is the enhanced proximity version.
// Buttons start moving away before the cursor even reaches them.

document.addEventListener("mousemove", (e) => {
  document.querySelectorAll(".flee-btn").forEach(btn => {
    const rect = btn.getBoundingClientRect();
    const cx = rect.left + rect.width / 2;
    const cy = rect.top  + rect.height / 2;
    const dx = e.clientX - cx;
    const dy = e.clientY - cy;
    const dist = Math.sqrt(dx * dx + dy * dy);

    if (dist < 80) {
      // Push the button away from the cursor
      const push = (80 - dist) / 80;
      const nx = -dx * push * 2;
      const ny = -dy * push * 2;
      btn.style.transform = `translate(${nx}px, ${ny}px)`;
      btn.style.transition = "transform 0.08s ease-out";
    } else {
      btn.style.transform = "";
      btn.style.transition = "transform 0.4s ease-out";
    }
  });
});

// ── CRT ambient video: gently flicker every 45–90s ──────────────────────────

function startCrtFlicker() {
  const video = document.getElementById("ambient-video");
  if (!video) return;

  function scheduleFlicker() {
    const delay = 45_000 + Math.random() * 45_000;
    setTimeout(() => {
      video.style.opacity = "0";
      setTimeout(() => {
        video.style.opacity = "1";
        scheduleFlicker();
      }, 80 + Math.random() * 120);
    }, delay);
  }
  video.style.transition = "opacity 0.04s";
  scheduleFlicker();
}
window.addEventListener("load", startCrtFlicker);
