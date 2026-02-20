-- 1. Пользователи (login/password/balance без jwt_token)
CREATE TABLE users (
                       id SERIAL PRIMARY KEY,
                       name TEXT NOT NULL,
                       login VARCHAR(50) UNIQUE NOT NULL,
                       password_hash VARCHAR(255) NOT NULL,
    -- баланс в центах/копейках
                       balance BIGINT NOT NULL DEFAULT 0
);

-- Новая таблица для сессий
CREATE TABLE sessions (
                          session_id TEXT PRIMARY KEY,  -- Задается из кода, не SERIAL
                          user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                          refresh_hash TEXT NOT NULL,
                          expired_time TIMESTAMP NOT NULL
);

-- 2. Состояние игры «Line Slots» (обычные 5x3 слоты)
CREATE TABLE line_game_state (
                                 user_id INT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
                                 free_spins_count INT NOT NULL DEFAULT 0,
                                 wild_data INT[][] NOT NULL DEFAULT '{}'::int[][]
);

-- 3. Состояние игры «Sugar Rush» (cascade-механика с множителями 7x7)
CREATE TABLE sugar_rush_state (
                                  user_id INT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
                                  free_spins_count INT NOT NULL DEFAULT 0,

    -- Храним множители и hits как JSONB — это самое удобное и быстрое решение
                                  multipliers JSONB NOT NULL DEFAULT '[[1,1,1,1,1,1,1],[1,1,1,1,1,1,1],[1,1,1,1,1,1,1],[1,1,1,1,1,1,1],[1,1,1,1,1,1,1],[1,1,1,1,1,1,1],[1,1,1,1,1,1,1]]'::jsonb,
                                  hits JSONB NOT NULL DEFAULT '[[0,0,0,0,0,0,0],[0,0,0,0,0,0,0],[0,0,0,0,0,0,0],[0,0,0,0,0,0,0],[0,0,0,0,0,0,0],[0,0,0,0,0,0,0],[0,0,0,0,0,0,0]]'::jsonb
);
