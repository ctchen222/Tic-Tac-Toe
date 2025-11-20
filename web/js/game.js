// web/js/game.js
// This file contains game-specific logic, WebSocket communication, and UI updates related to the game board.

// DOM Elements (These will be passed from main.js or accessed globally after page load)
let mainStatusElem;
let statusTextElem;
let lobbyAreaElem;
let gameAreaElem;
let currentTurnDisplayElem;
let gameMessageElem;
let gameBoardElem;
let playBotBtn;
let playHumanBtn;
let difficultySelect;
let rematchButtonsElem;
let rematchYesBtn;
let rematchNoBtn;
let leaveLobbyBtn;

// Game State
let ws;
let currentPlayerMark = '';
let isMyTurn = false;

export function setGameDOMElements(elements) {
    mainStatusElem = elements.mainStatusElem;
    statusTextElem = elements.statusTextElem;
    lobbyAreaElem = elements.lobbyAreaElem;
    gameAreaElem = elements.gameAreaElem;
    currentTurnDisplayElem = elements.currentTurnDisplayElem;
    gameMessageElem = elements.gameMessageElem;
    gameBoardElem = elements.gameBoardElem;
    playBotBtn = elements.playBotBtn;
    playHumanBtn = elements.playHumanBtn;
    difficultySelect = elements.difficultySelect;
    rematchButtonsElem = elements.rematchButtonsElem;
    rematchYesBtn = elements.rematchYesBtn;
    rematchNoBtn = elements.rematchNoBtn;
    leaveLobbyBtn = elements.leaveLobbyBtn;
}

export function renderBoard(board) {
    if (!board) return;
    const cells = gameBoardElem.children;
    let k = 0;
    for (let r = 0; r < 3; r++) {
        for (let c = 0; c < 3; c++) {
            const mark = board[r][c];
            cells[k].textContent = mark;
            cells[k].className = `cell ${mark ? mark.toLowerCase() : ''}`;
            k++;
        }
    }
}

export function sendMove(index) {
    console.log('sendMove called with index:', index);
    console.log('ws:', ws);
    console.log('ws.readyState:', ws ? ws.readyState : 'ws is null');
    console.log('isMyTurn:', isMyTurn);

    if (ws && ws.readyState === WebSocket.OPEN && isMyTurn) {
        const row = Math.floor(index / 3);
        const col = index % 3;
        const moveMsg = { type: 'move', position: [row, col] };
        console.log('Sending move:', moveMsg);
        ws.send(JSON.stringify(moveMsg));
    } else {
        console.log('Cannot send move. Conditions not met.');
    }
}

export function sendRematchVote(accept) {
    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'rematch', accept: accept }));
        rematchButtonsElem.style.display = 'none';
    }
}

export function connectWebSocket(token) {
    if (!token) {
        statusTextElem.className = 'status-message error-message';
        statusTextElem.textContent = '驗證失敗，無法取得權杖。';
        return;
    }

    statusTextElem.textContent = '嘗試連線中...';
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${protocol}//${window.location.host}/api/ws?token=${token}`);

    ws.onopen = () => {
        statusTextElem.textContent = '連線成功，正在進入大廳...';
    };

    ws.onmessage = (event) => {
        const msg = JSON.parse(event.data);
        console.log('Received:', msg);

        switch (msg.type) {
            case 'lobby_joined':
                mainStatusElem.style.display = 'none';
                lobbyAreaElem.style.display = 'block';
                gameAreaElem.style.display = 'none';
                rematchButtonsElem.style.display = 'none'; // Hide rematch buttons on returning to lobby
                break;

            case 'queue_joined':
                lobbyAreaElem.style.display = 'none';
                mainStatusElem.style.display = 'block';
                statusTextElem.textContent = '已加入佇列，等待對手中...';
                break;

            case 'assignment':
                lobbyAreaElem.style.display = 'none';
                mainStatusElem.style.display = 'none';
                gameAreaElem.style.display = 'block';
                currentPlayerMark = msg.mark;
                gameMessageElem.textContent = `你是 ${currentPlayerMark}。`;
                break;

            case 'update':
                renderBoard(msg.board);
                if (msg.winner) {
                    currentTurnDisplayElem.textContent = '';
                    isMyTurn = false;
                    rematchButtonsElem.style.display = 'block';
                    gameMessageElem.textContent = msg.winner.toLowerCase() === "draw" ? `遊戲結束！平局！` : `遊戲結束！贏家是 ${msg.winner}！`;
                } else if (msg.isDraw) {
                    currentTurnDisplayElem.textContent = '';
                    isMyTurn = false;
                    rematchButtonsElem.style.display = 'block';
                    gameMessageElem.textContent = `遊戲結束！平局！`;
                } else if (msg.next) {
                    currentTurnDisplayElem.textContent = `現在輪到 ${msg.next} 下棋。`;
                    isMyTurn = (msg.next === currentPlayerMark);
                    gameMessageElem.textContent = isMyTurn ? `輪到你 (${currentPlayerMark}) 下棋。` : `輪到對手 (${msg.next}) 下棋。`;
                }
                break;

            case 'error':
                gameMessageElem.textContent = `錯誤: ${msg.message}`;
                break;

            case 'rematch_request':
                gameMessageElem.textContent = '對手請求重賽！';
                rematchButtonsElem.style.display = 'block';
                break;

            case 'rematch_successful':
                rematchButtonsElem.style.display = 'none';
                renderBoard([["", "", ""], ["", "", ""], ["", "", ""]]);
                currentPlayerMark = '';
                isMyTurn = false;
                currentTurnDisplayElem.textContent = '';
                break;

            case 'opponent_left':
                gameMessageElem.textContent = '對手已離開房間，即將返回大廳...';
                rematchButtonsElem.style.display = 'none';
                // The server will follow up with a 'lobby_joined' message to trigger the UI change
                break;

            case 'opponent_disconnected':
                gameMessageElem.textContent = '對手已斷線，等待重連中...';
                break;

            case 'opponent_reconnected':
                gameMessageElem.textContent = '對手已重新連線！';
                break;

            default:
                console.log('Unknown message type:', msg.type);
        }
    };

    ws.onclose = () => {
        mainStatusElem.style.display = 'block';
        lobbyAreaElem.style.display = 'none';
        gameAreaElem.style.display = 'none';
        statusTextElem.className = 'status-message error-message';
        statusTextElem.textContent = '連線中斷... 5秒後嘗試重連...';
        setTimeout(() => {
            connectWebSocket(token);
        }, 5000);
    };

    ws.onerror = () => {
        statusTextElem.className = 'status-message error-message';
        statusTextElem.textContent = 'WebSocket 連線錯誤！';
    };
}

export function initGamePage() {
    // Event Listeners for game mode selection
    playBotBtn.onclick = () => {
        const difficulty = difficultySelect.value;
        ws.send(JSON.stringify({ type: 'start_bot_game', difficulty: difficulty }));
        lobbyAreaElem.style.display = 'none';
        mainStatusElem.style.display = 'block';
        statusTextElem.textContent = '正在建立機器人對戰...';
    };

    playHumanBtn.onclick = () => {
        ws.send(JSON.stringify({ type: 'join_queue' }));
    };

    // Event Listeners for game board cells
    gameBoardElem.addEventListener('click', (event) => {
        console.log('Board clicked!');
        const cell = event.target.closest('.cell');
        console.log('Clicked cell:', cell);
        if (cell) {
            console.log('cell.dataset.index:', cell.dataset.index);
        }
        if (cell && cell.dataset.index) {
            sendMove(parseInt(cell.dataset.index, 10));
        }
    });

    // Event Listeners for rematch buttons
    rematchYesBtn.onclick = () => sendRematchVote(true);
    rematchNoBtn.onclick = () => sendRematchVote(false);
    leaveLobbyBtn.onclick = () => {
        if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: 'leave_room' }));
        }
    };
}