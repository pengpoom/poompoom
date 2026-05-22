const tiltTarget = document.getElementById("mockupTilt");
const dashboard = tiltTarget?.querySelector(".dashboard");
const overlay = document.getElementById("authOverlay");
const modalTriggers = document.querySelectorAll("[data-modal-open]");
const closeTriggers = document.querySelectorAll("[data-modal-close]");
const heroPage = document.querySelector(".hero-page");
const workspacePage = document.getElementById("workspacePage");
const loginEmailForm = document.getElementById("loginEmailForm");
const loginCodeForm = document.getElementById("loginCodeForm");
const loginEmailInput = document.getElementById("loginEmail");
const loginEmailPreview = document.getElementById("loginEmailPreview");
const signupForm = document.getElementById("signupForm");
const enterWorkspaceButtons = document.querySelectorAll("[data-enter-workspace]");
const composerForm = document.querySelector(".composer");
const modals = {
  login: document.getElementById("loginModal"),
  signup: document.getElementById("signupModal"),
};
let activeModal = null;

function setStep(selector, activeValue, attributeName) {
  document.querySelectorAll(selector).forEach((step) => {
    step.classList.toggle("is-current", step.getAttribute(attributeName) === activeValue);
  });
}

function setTilt(event) {
  if (!tiltTarget || !dashboard) return;

  const bounds = tiltTarget.getBoundingClientRect();
  const x = (event.clientX - bounds.left) / bounds.width - 0.5;
  const y = (event.clientY - bounds.top) / bounds.height - 0.5;
  const rotateY = x * 8;
  const rotateX = y * -6;

  dashboard.style.setProperty("--tilt-x", `${rotateX.toFixed(2)}deg`);
  dashboard.style.setProperty("--tilt-y", `${rotateY.toFixed(2)}deg`);
}

function resetTilt() {
  if (!dashboard) return;

  dashboard.style.setProperty("--tilt-x", "0deg");
  dashboard.style.setProperty("--tilt-y", "0deg");
}

if (tiltTarget && dashboard && !window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
  tiltTarget.addEventListener("pointermove", setTilt);
  tiltTarget.addEventListener("pointerleave", resetTilt);
  tiltTarget.addEventListener("pointercancel", resetTilt);
}

function openModal(type) {
  if (!overlay || !modals[type]) return;

  Object.values(modals).forEach((modal) => modal?.classList.remove("is-active"));
  setStep("[data-login-step]", "email", "data-login-step");
  setStep("[data-signup-step]", "form", "data-signup-step");
  activeModal = modals[type];
  activeModal.classList.add("is-active");
  overlay.classList.add("is-open");
  overlay.setAttribute("aria-hidden", "false");
  document.body.classList.add("modal-open");

  const firstInput = activeModal.querySelector("input");
  window.setTimeout(() => firstInput?.focus(), 120);
}

function closeModal() {
  if (!overlay) return;

  overlay.classList.remove("is-open");
  overlay.setAttribute("aria-hidden", "true");
  document.body.classList.remove("modal-open");
  Object.values(modals).forEach((modal) => modal?.classList.remove("is-active"));
  activeModal = null;
}

modalTriggers.forEach((trigger) => {
  trigger.addEventListener("click", () => {
    openModal(trigger.dataset.modalOpen);
  });
});

closeTriggers.forEach((trigger) => {
  trigger.addEventListener("click", closeModal);
});

overlay?.addEventListener("click", (event) => {
  if (event.target === overlay) closeModal();
});

document.addEventListener("keydown", (event) => {
  if (event.key === "Escape" && activeModal) closeModal();
});

document.querySelectorAll(".auth-form").forEach((form) => {
  form.addEventListener("submit", (event) => {
    event.preventDefault();
  });
});

loginEmailForm?.addEventListener("submit", () => {
  const email = loginEmailInput?.value.trim() || "your email";
  if (loginEmailPreview) loginEmailPreview.textContent = email;
  setStep("[data-login-step]", "code", "data-login-step");
  window.setTimeout(() => document.getElementById("loginCode")?.focus(), 80);
});

loginCodeForm?.addEventListener("submit", () => {
  enterWorkspace();
});

document.querySelector("[data-login-back]")?.addEventListener("click", () => {
  setStep("[data-login-step]", "email", "data-login-step");
  window.setTimeout(() => loginEmailInput?.focus(), 80);
});

signupForm?.addEventListener("submit", () => {
  setStep("[data-signup-step]", "success", "data-signup-step");
});

function enterWorkspace() {
  closeModal();
  document.body.classList.add("workspace-active");
  heroPage?.classList.add("is-hidden");
  if (workspacePage) {
    workspacePage.setAttribute("aria-hidden", "false");
    workspacePage.scrollIntoView({ block: "start" });
  }
}

enterWorkspaceButtons.forEach((button) => {
  button.addEventListener("click", enterWorkspace);
});

composerForm?.addEventListener("submit", (event) => {
  event.preventDefault();
});
