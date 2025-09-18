# 📘 Przewodnik użytkownika Kup Piksel

Poniższy dokument opisuje sposób uruchomienia projektu lokalnie (tryb deweloperski i testowy) oraz kroki potrzebne do wdrożenia aplikacji na serwer VPS.

---

## 1. Wymagania wstępne

### Oprogramowanie
- **Docker** i **Docker Compose** (rekomendowane do szybkiego startu lokalnego oraz na VPS).
- Alternatywnie do pracy bez Dockera:
  - **Node.js 18+** (dla frontendu Vite/React).
  - **Go 1.21+** (dla backendu).

### Struktura repozytorium
- `frontend/` – aplikacja React (Vite + Tailwind CSS).
- `backend/` – serwer API w Go (framework Gin) oraz osadzony build frontendu.
- `docker-compose.yml` + `Dockerfile` – kompletna konfiguracja obrazu łączącego frontend i backend.

---

## 2. Uruchomienie lokalne (Docker Compose)

1. **Zbuduj i uruchom kontenery**:
   ```bash
   docker compose up --build
   ```
   Po skompilowaniu aplikacja będzie dostępna pod adresem `http://localhost:3000`.

2. **Test smoke – sprawdzenie API**:
   ```bash
   curl http://localhost:3000/api/pixels | head
   ```
   Otrzymasz początek odpowiedzi JSON z bieżącym stanem pikseli.

3. **Zatrzymanie**:
   ```bash
   docker compose down
   ```

> 💡 Docker Compose korzysta z wieloetapowego builda (Node → Go → Alpine). Dzięki temu uzyskujemy jeden lekki kontener produkcyjny.

---

## 3. Uruchomienie lokalne (tryb deweloperski bez Dockera)

Możesz uruchomić frontend i backend osobno, aby mieć szybkie odświeżanie kodu.

### 3.1 Backend (Go)
1. Zainstaluj zależności (moduły Go zostaną pobrane automatycznie przy starcie).
2. Uruchom serwer:
   ```bash
   cd backend
   go run .
   ```
   Backend nasłuchuje na `http://localhost:3000`.

### 3.2 Frontend (Vite)
1. Zainstaluj paczki NPM:
   ```bash
   cd frontend
   npm install
   ```
2. Uruchom serwer deweloperski Vite:
   ```bash
   npm run dev
   ```
   Domyślnie frontend będzie dostępny na `http://localhost:5173` i będzie proxował zapytania API na port 3000 (konfiguracja w `vite.config.ts`).

### 3.3 Testowanie
- **Test manualny**: otwórz aplikację w przeglądarce i spróbuj zaznaczyć piksele.
- **Test API**: np. update piksela
  ```bash
  curl -X POST http://localhost:3000/api/pixels \
       -H "Content-Type: application/json" \
       -d '{"id":42,"status":"taken","color":"#123456","url":"https://example.com"}'
  ```

> ℹ️ Po zmianie frontendu warto wykonać `npm run build` (patrz sekcja 4), by upewnić się że kompilacja produkcyjna przechodzi bez błędów.

---

## 4. Budowanie artefaktów

### 4.1 Build frontendu
```bash
cd frontend
npm run build
```
Wynik pojawi się w katalogu `frontend/dist`. Podczas builda Dockera folder ten jest kopiowany i osadzany w binarce Go.

### 4.2 Build backendu
```bash
cd backend
CGO_ENABLED=0 go build -o kup-piksel ./...
```
Artefakt `kup-piksel` zawiera już osadzony frontend (jeśli katalog `frontend_dist` istnieje).

---

## 5. Wdrożenie na VPS (produkcja)

Scenariusz zakłada pojedynczy serwer z systemem Linux oraz dostępem SSH.

1. **Zainstaluj Dockera i Docker Compose** (np. `apt install docker.io docker-compose-plugin`).
2. **Sklonuj repozytorium** lub skopiuj artefakty na serwer:
   ```bash
   git clone https://github.com/<twoja-organizacja>/KupPixel.git
   cd KupPixel
   ```
3. **Skonfiguruj zmienne środowiskowe** (jeżeli w przyszłości dodasz np. bazę danych). Na razie `docker-compose.yml` działa bez dodatkowych sekretów.
4. **Zbuduj i uruchom kontener** w trybie detached:
   ```bash
   docker compose up --build -d
   ```
5. **Sprawdź logi**:
   ```bash
   docker compose logs -f
   ```
6. **Ustaw reverse proxy / SSL** (opcjonalnie):
   - Zainstaluj Nginx lub Traefik.
   - Skonfiguruj proxy na port `3000` kontenera.
   - Dodaj certyfikaty SSL (np. Let’s Encrypt + certbot).
7. **Aktualizacja aplikacji**:
   ```bash
   git pull
   docker compose pull   # jeśli korzystasz z gotowych obrazów
   docker compose up --build -d
   ```
8. **Backup**: ponieważ stan pikseli jest obecnie trzymany w pamięci, po restarcie startujemy „na czysto”. Przy wdrażaniu produkcyjnym warto podłączyć zewnętrzną bazę danych (Redis/Postgres) i dodać wolumeny.

---

## 6. Dodatkowe wskazówki
- Monitoruj zasoby kontenera: `docker stats`.
- Dodaj health-check (np. endpoint `/api/pixels`) w orkiestracji lub w przyszłości w Kubernetes.
- Przy automatyzacji CI/CD możesz budować obraz poleceniem:
  ```bash
  docker build -t registry.example.com/kup-piksel:latest .
  docker push registry.example.com/kup-piksel:latest
  ```
- W razie problemów z cache NPM w Dockerze, wyczyść katalog `/root/.npm` lub usuń parametry cache w Dockerfile.

---

## 7. Kontakt i wsparcie
- Issues i pull requesty na GitHubie.
- Dodaj kanał komunikacji (Slack/Discord) dla zespołu, aby reagować na zgłoszenia i incydenty.

Powodzenia w pracy z Kup Piksel! 🎉
