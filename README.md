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
- **Baza danych**: na start in-memory (mapa pikseli), docelowo Redis/Mongo/Postgres  
- **Docker**: multi-stage build → jeden image z frontendem i backendem  
- **Nginx/Reverse Proxy**: opcjonalnie do hostingu na VPS + SSL  

