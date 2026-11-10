<p align="center">
  <img src="packaging/icons/128.png" alt="Apogée" width="128" height="128">
</p>

<h1 align="center">Apogée</h1>

<p align="center"><em>Souffle tes intentions à ton peuple. Porte-le au plus haut, avant sa chute.</em></p>

Apogée est un jeu de stratégie passive, pensé pour un second écran. Tu es un dieu distant : tu ne commandes pas ton peuple, tu lui souffles des intentions (prospérer, se fortifier, s'armer, étudier, s'étendre), et il décide lui-même comment les réaliser. Ton île flotte avec celles des autres joueurs autour d'un portail d'où surgissent des monstres. Avec l'âge et la puissance, elle descend vers lui, jusqu'à sa chute, puis ton peuple renaît.

Le ciel vit en continu sur un serveur, même quand le jeu est fermé. Tout le rendu (pixel art à facettes) et le son (chiptune) sont générés par le code.

## Installer

Les [releases](https://github.com/Bornholm/apogee/releases) fournissent, pour Linux amd64 et arm64 :

- des paquets Arch Linux : `sudo pacman -U apogee-*.pkg.tar.zst`, plus `apogee-server-*.pkg.tar.zst` pour un serveur dédié ;
- des archives `.tar.gz`, avec un `INSTALL.md` pour les autres distributions.

## Jouer

Lance `apogee`. L'écran d'accueil propose :

- une partie solo, avec des dieux rivaux joués par l'ordinateur ;
- d'héberger un ciel sur le réseau local, que tes collègues rejoignent ;
- de rejoindre un ciel trouvé sur le réseau, ou par son adresse.

Un tutoriel accompagne la première partie. En jeu : la molette zoome, les flèches font défiler la vue, **C** ouvre la culture et le savoir, **A** les Annales, **H** le tutoriel, **M** coupe le son.

Pour un ciel permanent, le paquet `apogee-server` installe un service systemd :

```sh
sudo systemctl enable --now apogee-server
```

Son nom et ses options se règlent dans `/etc/apogee/server.env`.

## Développer

Il faut Go 1.27. Le `Makefile` réunit les commandes courantes, et `make` les liste :

```sh
make run      # lance le jeu
make check    # vet, formatage et tests
make fast     # une partie accélérée, pour voir une île grandir
make server   # un ciel dédié sur le réseau local
make balance  # le banc d'équilibrage automatisé
make build    # le client et le serveur dans bin/
```

Une release se publie en poussant un tag `v*` : GitHub Actions lance GoReleaser (`.goreleaser.yaml`).

Le [document de conception](GDD.md) décrit le jeu en détail, les choix faits et l'état d'avancement.
