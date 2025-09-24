# Konfiguracja Nginx (produkcyjna)

Ten katalog zawiera plik `nginx.conf` wykorzystywany na serwerze reverse proxy w środowisku produkcyjnym.

Plik odzwierciedla konfigurację kontenera Nginx, który terminując SSL rozsyła ruch na dwie aplikacje:

- `kineflow.pl` → upstream `kineflow_upstream` (kontener `app:3000`)
- `kuppixel.pl` → upstream `kuppixel_upstream` (kontener `kup-piksel:3000`)

Serwer HTTP na porcie 80 służy wyłącznie do przekierowań na HTTPS, a blok `catch-all` na 443 zwraca kod 444 dla nieobsługiwanych hostów.

Wszelkie zmiany w konfiguracji reverse proxy powinny być wprowadzane w tym pliku, a następnie wdrażane do kontenera Nginx.
