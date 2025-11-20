// web/js/auth.js
// This file handles user authentication (login, register) and token management.

import { loadPage } from "./main.js";

const TOKEN_KEY = "tic-tac-toe-token";

export function getToken() {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token) {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
}

async function handleAuthResponse(response, errorElem, username) {
  const data = await response.json();
  if (response.ok) {
    setToken(data.extras.token);
    localStorage.setItem("user_logged_in", "true"); // Flag for logged-in user
    localStorage.setItem("username", username); // Store username
    errorElem.style.display = "none";
    loadPage("game"); // Redirect to game page after successful auth
  } else {
    errorElem.textContent = data.extras.message || "驗證失敗";
    errorElem.style.display = "block";
  }
}

export function initLoginPage() {
  const loginForm = document.getElementById("loginForm");
  const loginErrorElem = document.getElementById("login-error");
  const switchToRegisterBtn = document.getElementById("switchToRegister");
  const guestLoginBtn = document.getElementById("guestLoginBtn");

  if (loginForm) {
    loginForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      const username = event.target.username.value;
      const password = event.target.password.value;

      try {
        const response = await fetch("/api/login", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ username, password }),
        });
        await handleAuthResponse(response, loginErrorElem, username);
      } catch (error) {
        loginErrorElem.textContent = "連線錯誤，請稍後再試。";
        loginErrorElem.style.display = "block";
      }
    });
  }

  if (guestLoginBtn) {
    guestLoginBtn.addEventListener("click", async () => {
      try {
        const response = await fetch("/api/guest-login", { method: "POST" });
        const data = await response.json();
        if (response.ok) {
          setToken(data.extras.token);
          localStorage.removeItem("user_logged_in"); // Ensure guest is not marked as registered user
          localStorage.removeItem("username");
          loginErrorElem.style.display = "none";
          loadPage("game");
        } else {
          loginErrorElem.textContent = data.extras.message || "訪客登入失敗。";
          loginErrorElem.style.display = "block";
        }
      } catch (error) {
        loginErrorElem.textContent = "連線錯誤，請稍後再試。";
        loginErrorElem.style.display = "block";
      }
    });
  }

  if (switchToRegisterBtn) {
    switchToRegisterBtn.onclick = (event) => {
      event.preventDefault();
      loadPage("register");
    };
  }
}

export function initRegisterPage() {
  const registerForm = document.getElementById("registerForm");
  const registerErrorElem = document.getElementById("register-error");
  const switchToLoginBtn = document.getElementById("switchToLogin");

  if (registerForm) {
    registerForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      const username = event.target.username.value;
      const password = event.target.password.value;
      const confirmPassword = event.target.confirm_password.value;

      if (password !== confirmPassword) {
        registerErrorElem.textContent = "密碼與確認密碼不符。";
        registerErrorElem.style.display = "block";
        return;
      }

      try {
        const response = await fetch("/api/register", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ username, password }),
        });
        await handleAuthResponse(response, registerErrorElem, username);
      } catch (error) {
        registerErrorElem.textContent = "連線錯誤，請稍後再試。";
        registerErrorElem.style.display = "block";
      }
    });
  }

  if (switchToLoginBtn) {
    switchToLoginBtn.onclick = (event) => {
      event.preventDefault();
      loadPage("login");
    };
  }
}

