PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
CREATE TABLE ai_models (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			provider TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP, custom_api_url TEXT DEFAULT '', custom_model_name TEXT DEFAULT '',
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);
INSERT INTO ai_models VALUES('deepseek','default','DeepSeek','deepseek',0,'','2025-11-03 09:09:52','2025-11-03 09:09:52','','');
INSERT INTO ai_models VALUES('qwen','default','Qwen','qwen',0,'','2025-11-03 09:09:52','2025-11-03 09:09:52','','');
CREATE TABLE exchange_secrets (
			exchange_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			credential_type TEXT NOT NULL,
			ciphertext BLOB NOT NULL,
			nonce BLOB NOT NULL,
			kms_ciphertext BLOB NOT NULL,
			kms_key_version TEXT NOT NULL,
			public_key_version TEXT NOT NULL,
			algorithm TEXT NOT NULL,
			aad BLOB NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (exchange_id, user_id, credential_type),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);
CREATE TABLE user_signal_sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			coin_pool_url TEXT DEFAULT '',
			oi_top_url TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id)
		);
CREATE TABLE traders (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			ai_model_id TEXT NOT NULL,
			exchange_id TEXT NOT NULL,
			initial_balance REAL NOT NULL,
			scan_interval_minutes INTEGER DEFAULT 3,
			is_running BOOLEAN DEFAULT 0,
			btc_eth_leverage INTEGER DEFAULT 5,
			altcoin_leverage INTEGER DEFAULT 5,
			trading_symbols TEXT DEFAULT '',
			use_coin_pool BOOLEAN DEFAULT 0,
			use_oi_top BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP, custom_prompt TEXT DEFAULT '', override_base_prompt BOOLEAN DEFAULT 0, is_cross_margin BOOLEAN DEFAULT 1, use_default_coins BOOLEAN DEFAULT 1, custom_coins TEXT DEFAULT '', system_prompt_template TEXT DEFAULT 'default',
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (ai_model_id) REFERENCES ai_models(id),
			FOREIGN KEY (exchange_id) REFERENCES exchanges(id)
		);
CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			otp_secret TEXT,
			otp_verified BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
CREATE TABLE system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
INSERT INTO system_config VALUES('coin_pool_api_url','','2025-11-03 09:09:52');
INSERT INTO system_config VALUES('btc_eth_leverage','5','2025-11-03 09:09:52');
INSERT INTO system_config VALUES('api_server_port','8080','2025-11-03 09:09:52');
INSERT INTO system_config VALUES('oi_top_api_url','','2025-11-03 09:09:52');
INSERT INTO system_config VALUES('stop_trading_minutes','60','2025-11-03 09:09:52');
INSERT INTO system_config VALUES('default_coins','["BTCUSDT","ETHUSDT","SOLUSDT","BNBUSDT","XRPUSDT","DOGEUSDT","ADAUSDT","HYPEUSDT"]','2025-11-03 09:09:52');
INSERT INTO system_config VALUES('altcoin_leverage','5','2025-11-03 09:09:52');
INSERT INTO system_config VALUES('beta_mode','true','2025-11-03 09:09:52');
INSERT INTO system_config VALUES('use_default_coins','true','2025-11-03 09:09:52');
INSERT INTO system_config VALUES('max_daily_loss','10.0','2025-11-03 09:09:52');
INSERT INTO system_config VALUES('jwt_secret','Qk0kAa+d0iIEzXVHXbNbm+UaN3RNabmWtH8rDWZ5OPf+4GX8pBflAHodfpbipVMyrw1fsDanHsNBjhgbDeK9Jg==','2025-11-03 09:09:52');
INSERT INTO system_config VALUES('admin_mode','false','2025-11-03 09:09:52');
INSERT INTO system_config VALUES('max_drawdown','20.0','2025-11-03 09:09:52');
INSERT INTO system_config VALUES('encryption_public_key',unistr('-----BEGIN PUBLIC KEY-----\u000aMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAxDsGHRSFXqR2YFoWMNWC\u000a8s0FlVE2KglHjLnm1f+i5yPfuTYkTUbVDu6RZuqLJdvhX+UO0x1XnwFIhZqmEfro\u000a8Myr5+RnItl7QGqWWcKry4ZlPHroMwIK50WJt316KUKVUv7wUMMLoUUq7yctI8V/\u000athRX+ZRaErJJU9DWkSqjYOVdc+KwsZnN9WifoYhp6veTKmJ1kJOd6AVtF+KJ/z0R\u000ahFarXjaQ89vf/oUgKahS/BUH7P6jpP+L+7z8G650oygp3Pn66eq+ttcUdc20WiBj\u000aK5eDBUJUUeNmdesqZXBafhJBhsQyilC0+LgI+3laSkGh3odMdY5Mf9lnke9mfX8E\u000aRQIDAQAB\u000a-----END PUBLIC KEY-----'),'2025-11-03 09:09:52');
INSERT INTO system_config VALUES('encryption_public_key_version','mock-v1','2025-11-03 09:09:52');
CREATE TABLE beta_codes (
			code TEXT PRIMARY KEY,
			used BOOLEAN DEFAULT 0,
			used_by TEXT DEFAULT '',
			used_at DATETIME DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
INSERT INTO beta_codes VALUES('2aw4wm',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('34cvds',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('3f39nc',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('3qmg67',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('5rjp6k',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('65a3e6',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('6hzgpr',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('6wruwb',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('8bdf7a',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('8jxnp5',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('8xp3xq',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('9r5uev',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('adbn7p',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('azm8y4',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('b6tfqu',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('bs32f9',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('ctz8gn',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('d8rmq8',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('dmf2yt',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('dz7e8d',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('e9ptrm',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('f25m8s',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('feuzgb',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('fnd7z7',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('h43s95',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('hgs7gq',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('huhkra',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('mhqch4',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('mqwkau',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('mwfssp',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('na7629',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('pb5c2n',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('q5k6jt',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('qrurb8',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('rssybm',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('s7hbk7',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('sj8rus',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('sxy53c',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('t8fjmk',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('udmqcb',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('um6xu6',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('uzwb4r',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('w2uh55',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('wejxcq',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('wtaama',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('x82qvu',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('ygg4d4',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('yv8hnn',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('z9ywv8',0,'',NULL,'2025-11-03 09:09:52');
INSERT INTO beta_codes VALUES('znpa5t',0,'',NULL,'2025-11-03 09:09:52');
CREATE TABLE IF NOT EXISTS "exchanges" (
			id TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			secret_key TEXT DEFAULT '',
			testnet BOOLEAN DEFAULT 0,
			hyperliquid_wallet_addr TEXT DEFAULT '',
			aster_user TEXT DEFAULT '',
			aster_signer TEXT DEFAULT '',
			aster_private_key TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id, user_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);
INSERT INTO exchanges VALUES('binance','default','Binance Futures','binance',0,'','',0,'','','','','2025-11-03 09:09:52','2025-11-03 09:09:52');
INSERT INTO exchanges VALUES('hyperliquid','default','Hyperliquid','hyperliquid',0,'','',0,'','','','','2025-11-03 09:09:52','2025-11-03 09:09:52');
INSERT INTO exchanges VALUES('aster','default','Aster DEX','aster',0,'','',0,'','','','','2025-11-03 09:09:52','2025-11-03 09:09:52');
CREATE TRIGGER update_users_updated_at
			AFTER UPDATE ON users
			BEGIN
				UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END;
CREATE TRIGGER update_ai_models_updated_at
			AFTER UPDATE ON ai_models
			BEGIN
				UPDATE ai_models SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END;
CREATE TRIGGER update_exchange_secrets_updated_at
			AFTER UPDATE ON exchange_secrets
			BEGIN
				UPDATE exchange_secrets 
				SET updated_at = CURRENT_TIMESTAMP 
				WHERE exchange_id = NEW.exchange_id AND user_id = NEW.user_id AND credential_type = NEW.credential_type;
			END;
CREATE TRIGGER update_traders_updated_at
			AFTER UPDATE ON traders
			BEGIN
				UPDATE traders SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END;
CREATE TRIGGER update_user_signal_sources_updated_at
			AFTER UPDATE ON user_signal_sources
			BEGIN
				UPDATE user_signal_sources SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END;
CREATE TRIGGER update_system_config_updated_at
			AFTER UPDATE ON system_config
			BEGIN
				UPDATE system_config SET updated_at = CURRENT_TIMESTAMP WHERE key = NEW.key;
			END;
CREATE TRIGGER update_exchanges_updated_at
			AFTER UPDATE ON exchanges
			BEGIN
				UPDATE exchanges SET updated_at = CURRENT_TIMESTAMP 
				WHERE id = NEW.id AND user_id = NEW.user_id;
			END;
COMMIT;
