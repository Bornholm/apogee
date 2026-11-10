<p align="center">
  <img src="packaging/icons/128.png" alt="Apogée" width="128" height="128">
</p>

<h1 align="center">Apogée</h1>

<p align="center"><em>Souffle tes intentions à ton peuple. Porte-le au plus haut, avant sa chute.</em></p>

Apogée est un jeu de stratégie passive, fait pour tourner sur un second écran pendant qu'on travaille.

Tu joues un dieu distant. Tu ne donnes aucun ordre. Tu répartis ton attention entre cinq intentions (prospérer, se fortifier, s'armer, étudier, s'étendre) et ton peuple choisit seul quoi bâtir et où. Ton île flotte avec celles des autres joueurs autour d'un portail. Des vagues de monstres en sortent. Plus ton île vieillit et gagne en puissance, plus elle descend vers ce portail, et elle finit toujours par y tomber. Ton peuple renaît alors sur une île neuve.

Une civilisation vit une semaine de temps réel au plus. Le serveur fait tourner le ciel en continu, jeu fermé ou non. Le jeu n'embarque aucune image ni aucun son. Il dessine ses îles en pixel art et compose sa musique 8 bits au lancement.

## Installer

Chaque [release](https://github.com/Bornholm/apogee/releases) contient, pour Linux amd64 et arm64, des paquets Arch et des archives `.tar.gz`.

Sur Arch Linux :

```sh
sudo pacman -U apogee-*.pkg.tar.zst
```

Le paquet `apogee-server-*.pkg.tar.zst` ajoute un serveur dédié. Sur les autres distributions, chaque archive contient un `INSTALL.md`.

## Jouer

Lance `apogee`. L'écran d'accueil permet de jouer seul contre des dieux rivaux pilotés par l'ordinateur, d'héberger un ciel que tes collègues rejoignent sur le réseau local, ou de rejoindre un ciel existant. Un tutoriel ouvre la première partie.

| Touche | Effet |
|---|---|
| Molette, + et − | Zoom |
| Flèches | Faire défiler la vue |
| I | État de la civilisation |
| S | Afficher ou masquer les souffles |
| C | Culture et savoir |
| A | Annales des civilisations tombées |
| H | Tutoriel |
| M | Couper le son |

Au lancement, puis toutes les 6 heures s'il reste ouvert, le jeu demande à GitHub si une version plus récente est sortie et, si c'est le cas, affiche le lien vers sa page. Cette requête ne transmet rien sur toi. `apogee -checkupdate=false` la désactive.

Pour un ciel qui tourne en permanence sur une machine du bureau, installe `apogee-server` puis active son service :

```sh
sudo systemctl enable --now apogee-server
```

Le nom du ciel et ses options se règlent dans `/etc/apogee/server.env`.

## Développer

Il faut Go 1.27. `make` liste les commandes du `Makefile`. Les plus utiles :

```sh
make run      # lance le jeu
make check    # vet, formatage et tests
make fast     # une partie à ×60, pour voir une île se bâtir
make server   # un ciel dédié sur le réseau local
make balance  # compare les stratégies sur des ciels simulés
make build    # le client et le serveur dans bin/
```

Pousser un tag `v*` publie une release. GitHub Actions lance alors GoReleaser avec `.goreleaser.yaml`.

Le [document de conception](GDD.md) détaille les règles, les décisions prises et l'avancement.

## Licence

Apogée est distribué sous [licence AGPL](LICENSE), version 3 ou ultérieure (AGPL-3.0-or-later). Si tu fais tourner une version modifiée du serveur pour d'autres joueurs, tu dois leur donner accès à son code source.

Les polices Pixelify Sans et Silkscreen, embarquées dans le jeu, sont sous licence SIL Open Font License 1.1 (`internal/client/fonts`).
