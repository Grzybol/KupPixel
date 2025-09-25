# 🟦 Kup Piksel

> Nowoczesna wersja kultowego projektu *Million Dollar Homepage* 🚀  
> Kupuj piksele, zostawiaj swoją reklamę i wspieraj nasz ekosystem serwerów Minecraft.

---

## 📖 Opis

**Kup Piksel** to aplikacja webowa pozwalająca użytkownikom na zakup miejsca reklamowego w postaci pojedynczych pikseli na interaktywnej tablicy o wielkości **1000 x 1000** (milion pikseli).  
Każdy piksel można kliknąć, zobaczyć czy jest wolny czy zajęty i w przyszłości podlinkować do własnej strony / grafiki.

---

## ✨ Funkcje (MVP)

- Wyświetlanie interaktywnej tablicy pikseli (frontend: React + Tailwind).
- API backendowe zarządzające stanem pikseli (Go + Gin).
- Kliknięcie wolnego piksela → placeholder strony „Kup ten piksel”.
- Kliknięcie zajętego piksela → przekierowanie na stronę docelową.
- Uruchamianie całości w Dockerze (`docker-compose up`).

---

## 🏗 Architektura

- **Frontend**: React + TypeScript + Tailwind CSS
- **Backend**: Go (Gin framework) – API REST do obsługi pikseli
- **Baza danych**: SQLite (plik tworzony domyślnie pod `backend/data/pixels.db`, można zmienić ścieżkę zmienną `PIXEL_DB_PATH`)
- **Docker**: multi-stage build → jeden image z frontendem i backendem
- **Nginx/Reverse Proxy**: opcjonalnie do hostingu na VPS + SSL (konfiguracja produkcyjna w `infra/nginx/nginx.conf`)

### ✉️ Konfiguracja backendu

Backend odczytuje ustawienia z pliku `config.json` (domyślnie w katalogu `backend/`, ścieżkę można nadpisać zmienną `PIXEL_CONFIG_PATH`). Format pliku to JSON/JSON5 – możesz korzystać z komentarzy i końcowych przecinków. Przykładowy plik znajdziesz pod `backend/config.example.json`.

Kluczowe opcje:

| Pole | Opis |
| --- | --- |
| `disableVerificationEmail` | Po ustawieniu na `true` nowi użytkownicy są automatycznie oznaczani jako zweryfikowani i nie są wysyłane żadne maile. |
| `email.language` | Ustala język wiadomości transakcyjnych (np. `pl` lub `en`) wykorzystywanych przy weryfikacji konta i resetowaniu haseł. |
| `passwordReset.baseUrl` | Opcjonalna baza URL używana do budowy linków resetujących hasło (domyślnie wartość zmiennej `PASSWORD_RESET_LINK_BASE_URL` lub adres weryfikacyjny). |
| `passwordReset.tokenTtlHours` | Liczba godzin, przez które link resetujący hasło pozostaje ważny. |
| `smtp` | (Opcjonalnie) konfiguracja transportu SMTP oparta na polach `host`, `port`, `username`, `password`, `fromEmail`, `fromName`. |

Jeżeli sekcja `smtp` jest pominięta lub pusta, backend pozostaje w trybie developerskim – link aktywacyjny pojawia się w logach (ConsoleMailer). Po poprawnym uzupełnieniu danych zostanie użyty prawdziwy serwer SMTP.

Reset haseł korzysta z endpointów `/api/password-reset/request` i `/api/password-reset/confirm`. Linki są budowane w oparciu o `passwordReset.baseUrl` (lub zmienną środowiskową `PASSWORD_RESET_LINK_BASE_URL`) i mają okres ważności określony przez `passwordReset.tokenTtlHours`.

### 🔐 Cloudflare Turnstile

Aby formularze mogły wyświetlać widżet Cloudflare Turnstile, należy skonfigurować zarówno frontend, jak i backend:

- **Frontend** oczekuje klucza publicznego w zmiennej `VITE_TURNSTILE_SITE_KEY`. Najprościej jest skopiować plik `frontend/.env.example` do `frontend/.env` i podmienić wartość na klucz z panelu Cloudflare. W przypadku budowania obrazu Dockera zmienna jest przekazywana jako argument `VITE_TURNSTILE_SITE_KEY`.
- **Backend** używa sekretu z pola `turnstileSecretKey` w pliku `config.json`. Wartość ta musi odpowiadać sekretowi wygenerowanemu dla tej samej witryny w Cloudflare, co klucz publiczny z frontendu.
- Dodatkowo można ustawić `VITE_TURNSTILE_DEBUG=true`, aby przesyłać szczegółowe zdarzenia debugujące widżetu do backendu (wysyłane są one na endpoint `/api/debug/turnstile`).

Brak którejkolwiek z powyższych wartości uniemożliwi poprawne działanie weryfikacji CAPTCHA.

### 📡 Logowanie do Elastic

Backend potrafi buforować logi i wysyłać je do klastra ElasticSearch poprzez API bulk. Konfiguracja odbywa się przez zmienne środowiskowe:

| Zmienna | Domyślna wartość | Opis |
| --- | --- | --- |
| `ELASTIC_URL` | `https://127.0.0.1:9200` | Adres klastra ElasticSearch. |
| `ELASTIC_INDEX` | `website-backend` | Nazwa indeksu, do którego trafiają logi. |
| `ELASTIC_API_KEY` | _(brak)_ | Wymagany klucz API (bez niego logowanie jest wyłączone). |
| `ELASTIC_VERIFY_CERT` | `false` | Czy weryfikować certyfikat TLS (przy self-signed ustaw na `false`). |
| `ELASTIC_CA_PATH` | _(brak)_ | Ścieżka do pliku CA, jeżeli `ELASTIC_VERIFY_CERT=true`. |
| `ELASTIC_FLUSH_MS` | `60000` | Odstęp między cyklicznymi flushami bufora (w milisekundach). |
| `ELASTIC_MAX_BUFFER` | `2000` | Maksymalna liczba wpisów w buforze przed wymuszeniem flush. |
| `ELASTIC_MAX_BYTES` | `5242880` | Maksymalny rozmiar bufora (w bajtach). |
| `ELASTIC_MAX_RETRIES` | `3` | Liczba ponowień wysyłki przy błędach 429/5xx. |

Klucz API warto przechowywać w pliku `.env` (np. `ELASTIC_API_KEY=xxxx`) lub w menedżerze sekretów i przekazywać go do kontenera. Logger rejestruje zdarzenia lokalnie na stderr oraz wysyła je do Elastica – przy zakończeniu procesu bufor jest automatycznie flushowany.

### 💾 Przechowywanie danych backendu

- Domyślny plik bazy: `backend/data/pixels.db` (tworzony automatycznie przy starcie backendu).
- Zmienna środowiskowa `PIXEL_DB_PATH` pozwala wskazać inną lokalizację pliku.
- W `docker-compose.yml` katalog `data/` jest montowany jako named volume (`pixel-data`), dzięki czemu baza nie resetuje się po przebudowaniu obrazu Dockera.
- Kopię zapasową najlepiej wykonywać po zatrzymaniu serwera (lub po `COMMIT`). Można też użyć polecenia `sqlite3 pixels.db ".backup backup.db"` na bieżącej instancji.

