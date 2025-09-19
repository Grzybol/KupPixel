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
- **Nginx/Reverse Proxy**: opcjonalnie do hostingu na VPS + SSL

### ✉️ Wysyłka e-maili

Backend potrafi wysyłać realne wiadomości SMTP z linkami aktywacyjnymi. W środowisku deweloperskim, gdy konfiguracja SMTP nie jest ustawiona, serwer loguje treść wiadomości (ConsoleMailer). Aby aktywować transport SMTP ustaw poniższe zmienne środowiskowe:

| Zmienna | Opis |
| --- | --- |
| `SMTP_HOST` | Adres serwera SMTP (np. `smtp.gmail.com`). |
| `SMTP_PORT` | Port serwera SMTP (np. `587`). |
| `SMTP_USERNAME` | Nazwa użytkownika konta SMTP (opcjonalna dla serwerów bez uwierzytelnienia, wymaga pary z hasłem). |
| `SMTP_PASSWORD` | Hasło/APi key do konta SMTP. |
| `SMTP_SENDER_EMAIL` | Adres nadawcy wiadomości (np. `noreply@twojadomena.pl`). |
| `SMTP_SENDER_NAME` | (Opcjonalnie) nazwa wyświetlana nadawcy. Domyślnie `Kup Piksel`. |

Jeśli dowolna z wymaganych wartości (`SMTP_HOST`, `SMTP_PORT`, `SMTP_SENDER_EMAIL`) jest pominięta, backend zapisze czytelny log i automatycznie wróci do trybu konsolowego.

### 💾 Przechowywanie danych backendu

- Domyślny plik bazy: `backend/data/pixels.db` (tworzony automatycznie przy starcie backendu).
- Zmienna środowiskowa `PIXEL_DB_PATH` pozwala wskazać inną lokalizację pliku.
- W `docker-compose.yml` katalog `data/` jest montowany jako named volume (`pixel-data`), dzięki czemu baza nie resetuje się po przebudowaniu obrazu Dockera.
- Kopię zapasową najlepiej wykonywać po zatrzymaniu serwera (lub po `COMMIT`). Można też użyć polecenia `sqlite3 pixels.db ".backup backup.db"` na bieżącej instancji.

