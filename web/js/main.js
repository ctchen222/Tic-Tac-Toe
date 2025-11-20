// web/js/main.js
// This is the main entry point for the frontend application.
// It handles page routing, global initialization, and authentication state.

import {
  getToken,
  clearToken,
  initLoginPage,
  initRegisterPage,
} from "./auth.js";
import { connectWebSocket, initGamePage, setGameDOMElements } from "./game.js";

// Global DOM Elements
const pageContent = document.getElementById("page-content");
const loginLink = document.getElementById("loginLink");
const registerLink = document.getElementById("registerLink");
const logoutLink = document.getElementById("logoutLink");
const usernameDisplay = document.getElementById("usernameDisplay");

// Global State
let wsInstance = null;

// Function to load page content dynamically
export async function loadPage(pageName) {
  try {
    const response = await fetch(`/pages/${pageName}.html`);
    if (!response.ok) {
      throw new Error(`Failed to load page: ${pageName}.html`);
    }
    const html = await response.text();
    pageContent.innerHTML = html;
    updateAuthLinks(); // Update links every time a page is loaded

    // Initialize page-specific scripts after content is loaded
    switch (pageName) {
      case "game":
        setGameDOMElements({
          mainStatusElem: document.getElementById("mainStatus"),
          statusTextElem: document.getElementById("statusText"),
          lobbyAreaElem: document.getElementById("lobbyArea"),
          gameAreaElem: document.getElementById("gameArea"),
          currentTurnDisplayElem: document.getElementById("currentTurnDisplay"),
          gameMessageElem: document.getElementById("gameMessage"),
          gameBoardElem: document.getElementById("gameBoard"),
          playBotBtn: document.getElementById("playBotBtn"),
          playHumanBtn: document.getElementById("playHumanBtn"),
          difficultySelect: document.getElementById("difficultySelect"),
          rematchButtonsElem: document.getElementById("rematchButtons"),
          rematchYesBtn: document.getElementById("rematchYesBtn"),
          rematchNoBtn: document.getElementById("rematchNoBtn"),
          leaveLobbyBtn: document.getElementById("leaveLobbyBtn"),
        });
        initGamePage();
        const token = getToken();
        if (token) {
          wsInstance = connectWebSocket(token);
        }
        break;
      case "login":
        initLoginPage();
        break;
      case "register":
        initRegisterPage();
        break;
      default:
        console.warn(`No initialization script for page: ${pageName}`);
    }
  } catch (error) {
    console.error(`Error loading page ${pageName}:`, error);
    pageContent.innerHTML = `<p class="error-message">無法載入頁面：${pageName}</p>`;
  }
}

// Function to update header links based on auth status
function updateAuthLinks() {
  const token = getToken();
  // A simple check to see if it's a guest or registered user
  // This could be improved by decoding the JWT
  const isGuest = !localStorage.getItem("user_logged_in"); // A simple flag

  if (token && !isGuest) {
    loginLink.style.display = "none";
    registerLink.style.display = "none";
    logoutLink.style.display = "inline-block";
    usernameDisplay.textContent = localStorage.getItem("username") || "已登入";
    usernameDisplay.style.display = "inline-block";
  } else {
    loginLink.style.display = "inline-block";
    registerLink.style.display = "inline-block";
    logoutLink.style.display = "none";
    usernameDisplay.style.display = "none";
  }
}

// Logout handler
logoutLink.onclick = (event) => {
  event.preventDefault();
  clearToken();
  localStorage.removeItem("user_logged_in");
  localStorage.removeItem("username");
  if (wsInstance) {
    wsInstance.close();
    wsInstance = null;
  }
  updateAuthLinks();
  loadPage("game"); // Go back to game page, which will trigger guest login
};

// Initial setup on page load
document.addEventListener("DOMContentLoaded", () => {
  // Set up event listeners for header links
  loginLink.onclick = (event) => {
    event.preventDefault();
    loadPage("login");
  };
  registerLink.onclick = (event) => {
    event.preventDefault();
    loadPage("register");
  };

  loadPage("login"); // Load the login page by default
});
