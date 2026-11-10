# Installer Apogée depuis l'archive

Sur Arch Linux, préférez les paquets `.pkg.tar.zst` de la release :

    sudo pacman -U apogee-*.pkg.tar.zst

## Le jeu (archive `apogee_…`)

    install -Dm755 apogee ~/.local/bin/apogee
    install -Dm644 apogee.desktop ~/.local/share/applications/apogee.desktop
    for s in 64 128 256; do
      install -Dm644 icons/$s.png ~/.local/share/icons/hicolor/${s}x${s}/apps/apogee.png
    done

Le jeu a besoin d'OpenGL, de X11 (ou XWayland) et d'ALSA (ou PipeWire et
PulseAudio, qui l'émulent). Sans périphérique audio : `apogee -sound=false`.

Au lancement, le jeu vérifie sur GitHub qu'il est à jour ; `apogee -checkupdate=false` désactive cette vérification.

## Le serveur dédié (archive `apogee-server_…`)

    sudo install -Dm755 apogee-server /usr/bin/apogee-server
    sudo install -Dm644 apogee-server.service /usr/lib/systemd/system/apogee-server.service
    sudo install -Dm644 server.env /etc/apogee/server.env
    sudo systemctl enable --now apogee-server

Le nom du ciel et les options se règlent dans `/etc/apogee/server.env` ; le
ciel est sauvegardé dans `/var/lib/apogee/sky.json`. Sans systemd :
`apogee-server -h`.
