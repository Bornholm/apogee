# 📘 GAME DESIGN DOCUMENT : *APOGÉE*

> **Version** 0.26 · 9 novembre 2026
> **Statut** : Phase 2 (alpha) en cours
> **Auteur** : GameForge, avec le concepteur
> **Cible** : développeur senior solo, Go + Ebitengine

> ⚠️ **Lecture du document.** Ce GDD consigne les décisions de conception. Les chiffres (durées, coûts, seuils) sont des **valeurs de départ à équilibrer**, signalées 🔧 : elles ne sont pas figées. L'annexe A tient le journal des décisions et les sujets encore ouverts.

---

# 1. VISION

## 1.1 Pitch

> **Un jeu de stratégie en temps réel passif, qui vit sur ton second écran. Tu es le dieu distant d'un peuple qui bâtit sa civilisation sur une île flottante. Tes collègues du même réseau local sont tes voisins, tes alliés de circonstance et tes cibles. Chaque île s'élève, puis finit aspirée par le tourbillon au centre du ciel : à toi de la porter le plus haut et le plus longtemps possible.**

## 1.2 Fantasme du joueur

*« Je lance le jeu le matin. Ma petite île travaille pendant que je travaille. Je jette un œil de temps en temps, je souffle une intention à mon peuple, je regarde la bataille se dérouler sur mon écran, et j'espère que mon collègue de l'étage du dessus ne va pas me piller pendant que je suis en réunion. »*

## 1.3 Les cinq piliers

| # | Pilier | Ce qu'il signifie | Ce qu'il interdit |
|---|---|---|---|
| 1 | **La victoire se décide avant la bataille** | Préparation, tech et choix stratégiques comptent plus que la réaction | Le micro-management de combat |
| 2 | **Un seul monde, une seule scène** | Toutes les îles partagent le même ciel, visible et zoomable | Les écrans de chargement, les instances séparées |
| 3 | **Ton île vit, même sans toi** | Le serveur fait vivre chaque île en continu. En l'absence de son dieu, le peuple suit les dernières intentions soufflées et tranche seul ses suppliques. Fermer le jeu ne protège pas l'île | Les récompenses de connexion quotidienne, les notifications culpabilisantes, toute protection obtenue en se déconnectant |
| 4 | **Des intentions, pas des instructions** | Le joueur est un **dieu distant** qui souffle des intentions à son peuple (section 6). Le peuple décide comment les réaliser, et sa culture oriente même sa technologie | Toute action directe : placer un bâtiment, cibler une unité, désigner un bâtiment |
| 5 | **Toute civilisation tombe, mais jamais par surprise** | La menace croît inexorablement à mesure qu'une île descend vers le portail, mais chaque étape s'annonce : indicateur de menace, présages, déclin visible. Ces signes existent dans le ciel et la chronique, **que le joueur soit là pour les voir ou non**. La difficulté dépend de l'âge, de la puissance, de la distance au portail et de la population, jamais de la performance du joueur | Les chutes sans présage, les pics de difficulté imprévisibles |

## 1.3 bis Thème : grandeur et chute

> **Le jeu est une métaphore de la montée et de la chute des civilisations.** Les vagues de monstres sont **toujours de plus en plus fortes** : toute île finit, tôt ou tard, par **chuter**.

| Principe | Détail |
|---|---|
| **Une chute littérale** | Quand son **cœur** est détruit, l'île est **aspirée par le portail**, le tourbillon géant au centre du ciel, et sa civilisation avec elle. Tout le ciel la voit sombrer (pilier 2). Le cycle se boucle : les monstres sortent du portail, les civilisations y finissent |
| **Une vie courte** | Une civilisation vit **au maximum une semaine de temps réel** 🔧. Son âge se compte en temps réel, que son joueur soit là ou non |
| **Le peuple se débrouille** | Le ciel ne s'arrête ni la nuit, ni le week-end, ni pendant les vacances. Une civilisation peut décliner et chuter pendant l'absence de son dieu : c'est le prix d'un peuple qui vit sans lui |
| **Une renaissance** | Après la chute, le joueur fonde une nouvelle civilisation en périphérie (11.5) |
| **Un objectif commun** | Monstres, rivaux IA et joueurs en raid ont le même but ultime : détruire le cœur d'une île |
| **Une spirale** | Les îles naissent en périphérie et se rapprochent du portail au fil de leur vie, où les vagues sont les plus violentes (3.1) |
| **Une fin inéluctable** | Le talent du joueur ne décide pas *si* sa civilisation tombera, mais **combien de temps** elle durera et **jusqu'où** elle s'élèvera |

Le pilier 5 traduit ce thème en règle : la chute est certaine, la surprise est interdite.

## 1.4 Public cible

| Segment | Description | Besoin principal |
|---|---|---|
| **Primaire** | Employés de bureau, développeurs, étudiants en résidence, joueurs sur un même LAN | Un jeu discret, social, sans engagement |
| **Secondaire** | Joueurs solo adeptes d'idle et de simulation | Une expérience complète sans autre humain |
| **Hors cible** | Joueurs compétitifs recherchant du skill mécanique | Ce jeu ne le servira pas |

## 1.5 Plateformes

Le jeu vise à terme **deux familles de plateformes**, qui jouent ensemble dans le même ciel. Elles arrivent **dans cet ordre** :

1. **Linux** : première et seule cible du MVP, de la vertical slice au lancement.
2. **Android, Windows et macOS** : ensemble, dans un second temps (phase 5).

| Plateforme | Distribution | Rôle |
|---|---|---|
| **Desktop** : Windows, macOS, Linux | Binaire unique | Fenêtre discrète sur un second écran. Peut lancer un **serveur intégré** (10.3) |
| **Android** | Application (APK ou AAB) | Le téléphone posé sur le bureau devient le second écran. **Jamais serveur** |

Règles communes :
- Une seule base de code Go et une seule simulation. Seules l'interface et l'intégration système diffèrent.
- Sur Android, une application mise en arrière-plan est suspendue par le système. Ça ne change rien pour l'île, qui vit sur le serveur (pilier 3).
- Dès le MVP Linux, l'interface reste **compatible avec le tactile** (aucune information réservée au survol), pour que le portage Android ne demande pas de la refaire (13.1).

## 1.6 Ce que le jeu n'est PAS

- ❌ Un RTS à micro-gestion
- ❌ Un jeu « free-to-play » avec temps d'attente monétisé
- ❌ Un jeu en ligne mondial
- ❌ Un jeu qui demande une attention continue

---

# 2. BOUCLE DE JEU

## 2.1 Les trois échelles de temps

| Échelle | Durée | Activité du joueur |
|---|---|---|
| **Micro** | 10 à 60 s | Regarder, zoomer, observer une vague ou un raid |
| **Méso** | 10 à 30 min | Lire la chronique, répondre à une supplique, ajuster le gâteau d'intentions, convoiter un voisin |
| **Macro** | Séance de 2 à 8 h (🔧) | Façonner la culture du peuple et donc sa technologie, préparer un boss |
| **Civilisation** | Jusqu'à une semaine | Mener une civilisation de sa naissance en périphérie à sa chute dans le portail, puis renaître (1.3 bis) |

## 2.2 Boucle principale

```
        ┌───────────────────────────────────────────────┐
        │                                               │
        ▼                                               │
   SOUFFLER ──► LE PEUPLE INTERPRÈTE ──► LA CULTURE SE FORME ──► DÉFENDRE / RAIDER
   (joueur)     (collecte, construit)     (oriente la tech)       (intentions + auto)
        ▲                                               │
        │                                               │
        └──────────── RÉCOMPENSES ◄─────────────────────┘
```

## 2.3 Principe fondamental : la simulation autonome

L'île **se joue seule**. Sans aucune intervention :
- le peuple collecte et construit selon le gâteau d'intentions de départ (6.2),
- les défenses repoussent les vagues faibles,
- le savoir progresse sur l'axe que dicte la culture,
- les suppliques sans réponse sont tranchées par le peuple lui-même.

Le joueur **oriente**, il ne pilote pas. Il n'a **jamais d'action directe** sur le jeu : comme un dieu distant, il souffle des intentions, et son peuple les interprète et les exécute. Un joueur absent laisse son peuple **livré à lui-même** : l'île continue de vivre sur le serveur, de produire, de se défendre, et peut être pillée ou même chuter (pilier 3, 7.5).

---

# 3. MONDE ET ÎLES

## 3.1 Le ciel

Un espace unique et ouvert, où flottent les îles de tous les participants.

> **Les îles sont en orbite autour d'un centre**, un **portail** en forme de **tourbillon géant**, d'où surgissent les monstres et les boss, et qui aspire les îles qui chutent.

| Principe | Détail |
|---|---|
| **Orbite** | Chaque île tourne lentement autour du portail. Les voisinages évoluent donc au fil du temps |
| **Exposition** | Plus une île est **proche du centre**, plus les vagues qui la visent sont **violentes** |
| **Naissance en périphérie** | Une nouvelle civilisation apparaît **au bord** du ciel, là où elle est le moins exposée |
| **Lisibilité** | La distance au portail montre d'un coup d'œil quelles îles sont les plus menacées (pilier 2) |

### La descente vers le portail

Au fil de sa vie, une île se rapproche du portail. Deux causes se cumulent :

| Cause 🔧 | Effet |
|---|---|
| **L'âge** | L'île descend lentement avec le temps réel |
| **La puissance** | Plus la civilisation est puissante (paliers, bâtiments, population), plus elle descend vite : plus elle s'élève, plus elle attire l'abîme |

Le joueur choisit entre **croître vite et mourir jeune**, ou **rester modeste et durer**. La descente est calibrée pour qu'une civilisation ne dépasse pas une semaine de temps réel (1.3 bis).

## 3.2 L'île du joueur

| Propriété | Valeur de départ 🔧 |
|---|---|
| Forme | **Contour polygonal procédural**, sans grille visible |
| Taille | Mesurée en **surface constructible** |
| Génération | Procédurale, déterminée par une **graine** unique par civilisation (une nouvelle à chaque renaissance, 11.5) |
| Contenu initial | 1 cœur central, 3 ouvriers, 2 sites de ressources |
| Expansion | Le contour **s'agrandit** quand le peuple s'étend (intention S'étendre, 6.3) |

## 3.3 Le cœur

Bâtiment central de l'île, qui la maintient dans le ciel. **Sa destruction fait chuter l'île** : elle est aspirée par le portail, avec sa civilisation (7.5). Le détruire est l'objectif ultime de toutes les menaces : monstres, rivaux IA et joueurs.

## 3.4 Le zoom

| Niveau | Ce qu'on voit | Utilité |
|---|---|---|
| **Ciel** | Toutes les îles, vagues et boss sous forme d'icônes | Vision stratégique |
| **Île** | Une île entière, bâtiments et unités | Gestion courante |
| **Détail** | Unités individuelles, animations | Plaisir de regarder |

On peut **observer n'importe quelle île** de près : on n'en voit que ce qui se voit (bâtiments, habitants, défenseurs), jamais ses stocks, son gâteau ni sa chronique.

Les trois niveaux sont des **niveaux de détail** progressifs dans une même scène, pas des écrans distincts. Le zoom se fait par **paliers entiers** (×1, ×2, ×3…) avec une transition animée entre deux paliers, et chaque vue correspond à un palier (12.3).

## 3.5 Placement des bâtiments

> **Le placement est libre**, sans grille, en fonction de l'espace disponible sur l'île. **C'est la simulation qui place**, jamais le joueur (pilier 4).

| Règle | Détail |
|---|---|
| **Emprise** | Chaque bâtiment occupe une forme simple (cercle ou petit polygone convexe) |
| **Validité** | Un emplacement est valide s'il est sur l'île et ne chevauche aucune emprise, avec une **marge minimale** 🔧 |
| **Autorité** | Le serveur valide chaque placement (voir 10.3) |
| **Portées et voisinage** | Bonus de proximité et portées de défense se mesurent en **rayon**, pas en cases |
| **Remparts** | Tracés comme une **ligne brisée** entre des points, avec une tour générée à chaque angle |
| **Qui place** | Les ouvriers choisissent l'emplacement selon des règles de la simulation (proximité des ressources, du cœur, des remparts) 🔧. Le joueur ne peut ni placer ni déplacer un bâtiment |

L'espace reste une ressource stratégique, mais le joueur ne la gère qu'indirectement : ses intentions orientent la façon dont son peuple occupe l'île (village tassé et productif, ou espacé et défendu). Le choix de *quoi* construire revient aussi au peuple, qui le déduit des intentions (6.4).

## 3.6 Cycle jour/nuit

Le ciel vit au rythme d'un **cycle de jeu d'environ 30 minutes** 🔧, **commun à toutes les îles** et donné par le serveur (pilier 2). Il n'est pas calé sur l'heure réelle : une journée de bureau traverse une quinzaine de nuits.

| Moment | Durée 🔧 |
|---|---|
| 🌅 **Aube** | 3 min |
| ☀️ **Jour** | 15 min |
| 🌇 **Crépuscule** | 3 min |
| 🌙 **Nuit** | 9 min |

> **Principe : un rythme léger et télégraphié.** Le cycle module le jeu sans jamais en changer les règles, et aucun moment ne prend le joueur par surprise (pilier 5).

| Effet 🔧 | Jour | Nuit | Voir |
|---|---|---|---|
| **Production** (collecte, construction) | Normale | Réduite (×0,6), le peuple dort en partie | 4.2 |
| **Savoir** | Normal | Accru (×1,3), les savants veillent | 5.3 |
| **Vagues** | Normales | Plus fortes (+20 %), **annoncées dès le crépuscule** | 8.2 |
| **Lisibilité des raids** | Normale | Silhouettes moins lisibles, sauf pour un peuple aux Arcanes avancées | 7.3 |
| **Suppliques** | Possibles | Arrivent de préférence **à l'aube** | 6.5 |

La nuit avantage légèrement la culture Arcanes (savoir, vision nocturne), ce qui donne un rôle concret à la « détection » de cet axe.

---

# 4. RESSOURCES ET ÉCONOMIE

## 4.1 Ressources de base

| Ressource | Source | Usage |
|---|---|---|
| **Pierre** | Sites de carrière | Construction, défenses |
| **Bois** | Bosquets | Construction, outils |
| **Vivres** | Fermes | Entretien des unités |
| **Aether** | Cristaux flottants | Technologie, unités avancées |

> 🔧 À valider en prototype : quatre ressources est un maximum. Si la lisibilité souffre, réduire à trois.

## 4.2 Principes économiques

| Principe | Description |
|---|---|
| **Flux continu** | Production lente et régulière, jamais de pics. Elle baisse la nuit (3.6) sans jamais s'arrêter, et continue en l'absence du joueur, selon les dernières intentions soufflées (pilier 3) |
| **Plafond de stockage** | Évite l'accumulation illimitée, **et protège contre le pillage total** (voir 7.4) |
| **Pas de monnaie premium** | Aucune monétisation de la progression |

## 4.3 Le plafond de stockage, mécanisme à double fonction

Chaque ressource a un plafond lié aux entrepôts. Quand il est atteint, la production est gaspillée. Conséquences :
- ça **oblige à dépenser**, donc à construire et à former des unités,
- ça **limite l'intérêt du pillage** contre un joueur qui dépense régulièrement.

## 4.4 Ouvriers et répartition du travail

Il n'y a **aucun réglage direct** de la collecte. Le peuple répartit le travail entre Bois, Pierre, Vivres et Aether selon ses **besoins**, et ses besoins découlent des intentions soufflées (6.4) : Se fortifier fait monter le besoin de pierre, Étudier celui d'Aether, etc.

---

# 5. TECHNOLOGIE

## 5.1 Principe

> **Le joueur ne choisit pas la technologie. Elle découle de ce qu'il souffle à son peuple.**

Quatre **axes** technologiques. La **culture** du peuple (6.6), façonnée par les intentions soufflées au fil du temps, détermine l'axe vers lequel se tournent ses savants. Le joueur oriente le savoir de sa civilisation, il ne sélectionne jamais une technologie dans un arbre. La progression est **lente et permanente** : le monde n'a pas de saisons (11.2).

## 5.2 Les quatre axes

Chaque axe correspond à une **famille d'intentions** (6.3). C'est la même table qui relie intentions, culture et technologie.

| Axe | Famille d'intentions | Identité | Style de jeu favorisé | Faiblesse |
|---|---|---|---|---|
| ⚔️ **Armes** | S'armer, Convoiter, Punir, Anéantir | Unités offensives puissantes | Pillard | Défenses fragiles |
| 🛡️ **Murailles** | Se fortifier, Secourir | Défenses et soutien | Défenseur | Peu d'options offensives |
| ⚙️ **Industrie** | Prospérer, S'étendre | Économie et production | Bâtisseur | Vulnérable aux raids précoces |
| 🔮 **Arcanes** | Étudier | Effets spéciaux, détection, magie | Opportuniste | Coût élevé en Aether |

## 5.3 Règles de progression

| Règle | Détail |
|---|---|
| **Axes au MVP** | Les **4 axes** existent dès la vertical slice : chacun porte une famille d'intentions, et la boucle intentions, culture, technologie a besoin des quatre |
| **Paliers** | **2 paliers** par axe au MVP (Éveil, Maîtrise), soit 16 découvertes. Le 3e palier (Excellence) arrive en bêta, soit 24 découvertes 🔧 |
| **Points de savoir** | Produits en continu à partir de l'Aether. L'intention **Étudier** augmente leur production, et la nuit aussi (3.6) |
| **Direction** | Les points vont à l'axe qui **domine la culture** du peuple. En cas d'égalité, le gâteau d'intentions courant (6.2) départage |
| **Changer d'axe** | Il faut faire évoluer la culture, ce qui prend du temps : c'est le coût naturel d'un changement de cap |
| **Plafond de savoir** | Un peuple ne peut maîtriser qu'un nombre limité de paliers au total, tous axes confondus (environ 3 sur 8 au MVP, 5 sur 12 en bêta 🔧) : il excelle dans un ou deux axes, jamais dans tous |
| **Oubli** | Quand la culture se tourne vers un nouvel axe et que le plafond est atteint, les savants **oublient** peu à peu les paliers de l'axe délaissé. La chronique le raconte (*« Les derniers maîtres bâtisseurs se sont éteints. »*) |
| **Découvertes** | À chaque palier, les savants proposent **deux découvertes** par **supplique** (6.5). Le dieu en choisit une. S'il ne répond pas, le peuple choisit selon sa culture |
| **Puissance** | Mesure composite de l'île : paliers acquis, bâtiments, population 🔧. Elle sert partout où le GDD parle de niveau ou de force d'une île : vitesse de descente (3.1), écart de pillage (7.4), calibrage des vagues (8.3) |

L'intention Étudier a donc deux effets : elle accélère tout le savoir, et, soufflée longtemps, elle tourne la culture vers les Arcanes.

## 5.4 Lisibilité visuelle des choix

L'axe dominant d'une île doit être **lisible d'un coup d'œil** en vue « Ciel » : couleur dominante, silhouette de bâtiments, emblème. C'est ce qui permet d'évaluer un voisin avant de décider de le raider (pilier 1).

---

# 6. INTENTIONS ET PEUPLE

## 6.1 Les unités

| Catégorie | Rôle | Exemple |
|---|---|---|
| **Ouvriers** | Collecte et construction | Mineur, bûcheron |
| **Défenseurs** | Protection de l'île | Garde, archer de muraille |
| **Assaillants** | Raids | Fantassin, sapeur |
| **Spécialistes** | Effets particuliers | Soigneur, éclaireur |

Chaque catégorie gagne un niveau d'unités par palier de l'axe technologique correspondant (5.2) : deux niveaux au MVP, trois en bêta.

## 6.2 Souffles et conviction

Le dieu ne donne pas d'ordres : il **souffle** des intentions, et son peuple les adopte **progressivement**.

| Règle 🔧 | Détail |
|---|---|
| **Le gâteau** | Au plus **trois intentions** tournées vers l'île, réparties en parts (par paliers de 10 %), pour un total de 100 % |
| **Conviction** | Chaque part fixe une **cible de conviction**. La conviction réelle du peuple glisse vers cette cible en **1 à 3 minutes** |
| **Inertie** | Le peuple ne change jamais d'avis d'un coup. Cette inertie remplace tout délai de réaction artificiel et rend le micro-management inutile sans rien interdire |
| **Gâteau de départ** | Prospérer 60 %, Se fortifier 40 % |
| **Intention extérieure** | **Une seule à la fois**, en plus du gâteau, avec une île pour cible |

## 6.3 Le vocabulaire des intentions

**Tournées vers l'île** (le gâteau) :

| Intention | Famille (5.2) | Ce que le peuple en fait |
|---|---|---|
| 🌾 **Prospérer** | Industrie | Fermes, entrepôts, collecte, nouveaux ouvriers |
| 🔭 **S'étendre** | Industrie | Agrandit le contour de l'île, exploite des cristaux éloignés |
| 🧱 **Se fortifier** | Murailles | Remparts, tours, défenseurs postés |
| ⚔️ **S'armer** | Armes | Casernes, assaillants, forge |
| 📜 **Étudier** | Arcanes | Plus de points de savoir, bâtiments de savants |

**Tournées vers les autres** (une seule à la fois) :

| Intention | Famille (5.2) | Ce que le peuple en fait |
|---|---|---|
| 👁️ **Convoiter** [île] | Armes | Rassemble ses assaillants et part en raid **quand il se sent prêt**, au vu de la force visible de la cible 🔧 |
| 🔥 **Punir** [île] | Armes | Part **tout de suite** en raid avec les assaillants disponibles |
| 💀 **Anéantir** [île] | Armes | Prépare longuement un assaut contre le **cœur** de la cible. Possible **seulement contre une île en déclin** (7.5) |
| 🤝 **Secourir** [île] | Murailles | Envoie chez le voisin les défenseurs disponibles au-delà d'une garde minimale 🔧 |
| 🏠 **Rappeler** | aucune | Toutes les unités rentrent, l'intention extérieure en cours est annulée |

L'équilibre offensif/défensif ne passe plus par des modes : il découle du dosage entre Se fortifier et S'armer.

## 6.4 L'interprétation par le peuple

Le peuple est une **IA à scores d'utilité** (même moteur que les rivaux, voir 9.3). La conviction donne le poids de chaque intention dans ses décisions : quoi construire, où le placer (3.5), quelle ressource collecter (4.4), quelles unités former.

L'interprétation dépend du **contexte**. Avec « Se fortifier » à 50 %, le peuple élève des tours du côté d'où viennent les vagues et des remparts face au voisin qui l'a pillé. Le dieu donne le *quoi*, le peuple trouve le *où* et le *comment*.

## 6.5 Les suppliques

De temps en temps, le peuple **s'adresse à son dieu** pour une décision qu'il n'arrive pas à prendre seul.

| Règle 🔧 | Détail |
|---|---|
| **Fréquence** | Environ une toutes les 10 à 20 minutes |
| **Forme** | Une question et **2 ou 3 réponses** |
| **Sans réponse** | Après quelques minutes, le peuple **choisit seul** selon sa culture, **sans pénalité**. C'est le cas de toutes les suppliques pendant une absence (pilier 3) |
| **Pas d'accumulation** | Une seule supplique en attente : une nouvelle remplace l'ancienne |
| **Moment** | De préférence **à l'aube**, comme un conseil du matin (3.6) |

Exemples :
- *« Les vivres manquent. Faut-il abattre le bosquet de l'est pour faire des champs ? »*
- *« L'île de nos voisins a été pillée. Allons-nous l'aider ? »*
- *« Nos savants hésitent entre deux découvertes. »* (choix technologique, voir 5.3)

Les suppliques donnent au joueur présent des occasions de décider, et le joueur occupé ne perd rien.

## 6.6 La culture

Ce que le dieu souffle souvent devient la **culture** de son peuple.

| Règle 🔧 | Détail |
|---|---|
| **Calcul** | Moyenne glissante lente des convictions, regroupées par famille (5.2), sur quelques heures |
| **Bonus** | La famille dominante rend ses bâtiments et unités moins chers (environ −15 %) |
| **Résistance** | Les intentions hors de la famille dominante gagnent en conviction plus lentement (environ ×0,7) |
| **Technologie** | La famille dominante détermine l'axe que suivent les savants (5.3) |
| **Suppliques** | Sans réponse du dieu, le peuple tranche selon sa culture |
| **Lisibilité** | La culture se voit depuis le ciel : bannières, couleurs, silhouette des bâtiments (5.4) |

La culture rend chaque île unique au fil du temps et empêche une répartition optimale unique : changer radicalement de cap coûte du temps.

## 6.7 La chronique

Chaque île tient une **chronique** : quelques lignes, écrites dans la langue du jeu, qui racontent ce que le peuple a fait et pourquoi.

> *Voulant se fortifier, ton peuple élève un rempart face à l'île Rouge.*
> *Les pillards de l'île Rouge ont été repoussés. Deux gardes sont tombés.*

En revenant devant l'écran, le joueur comprend en dix secondes ce qui s'est passé. Après une longue absence, la chronique ouvre sur un **résumé** des faits marquants (raids, vagues, entrée en déclin, chute). Si la civilisation a chuté, le joueur retrouve son récit complet dans les Annales (11.5). Dans la scène, de petits signes (fumée de chantier, bannière d'intention au-dessus des bâtiments) montrent à quoi travaille le peuple. C'est la réponse au risque « mon peuple ne m'écoute pas ».

## 6.8 Règles de contrôle

| Règle | Justification |
|---|---|
| Aucune action directe : ni placer, ni cibler une unité, ni désigner un bâtiment | Pilier 4 |
| Une intention extérieure cible une **île**, jamais un bâtiment | Garde la lisibilité |
| Une seule intention extérieure à la fois | Simplicité |
| Toute intention passe par l'**inertie de conviction** (6.2) | Évite le « micro caché » |

## 6.9 Combat

Le combat est **automatique et en temps réel**, façon autobattler : les unités choisissent leurs cibles selon leur type. Le joueur influence l'issue **par ce qu'il a soufflé avant** (gâteau, culture, technologie), pas par sa réaction.

---

# 7. PvP : LES RAIDS

## 7.1 Principe

Un joueur peut lancer un raid sur n'importe quelle autre île du ciel, que son joueur soit connecté ou non (pilier 3). Le raid se déroule **en temps réel, dans le monde partagé**. Les deux parties peuvent le regarder.

## 7.2 Déroulement

1. **Déclaration** : le dieu souffle **Convoiter**, **Punir** ou **Anéantir** contre une île (6.3). Avec Punir, les assaillants partent aussitôt. Avec Convoiter, le peuple se prépare et part quand il se sent prêt. Anéantir, réservé aux îles en déclin, suit ses propres règles (7.5).
2. **Convoitise visible** : dès que Convoiter est soufflé, l'île qui convoite dresse une **bannière tournée vers sa cible**, visible depuis le ciel. La cible en est informée par sa chronique (*« L'île Rouge nous regarde avec envie. »*). Elle a ainsi le temps de souffler Se fortifier. Punir ne donne aucun signe préalable, mais part avec les assaillants disponibles, sans préparation : c'est le prix de la surprise.
3. **Trajet** : les unités traversent le ciel, visibles de tous.
4. **Alerte** : le défenseur est notifié **dès le départ**, dans le jeu et en notification système (13.1).
5. **Combat** : résolution automatique à l'arrivée.
6. **Résolution** : pillage limité (7.4), retraite ou destruction des assaillants, ou **chute** de l'île si son cœur est détruit (7.5).

## 7.3 Information et équité

| Principe | Application |
|---|---|
| Alerte systématique | Le défenseur sait toujours qu'une attaque arrive |
| Convoitise visible | Une île convoitée le sait avant même la préparation du raid (7.2) |
| Pillage seulement | Convoiter et Punir ne peuvent jamais atteindre le cœur : seul Anéantir le peut (7.5) |
| Visibilité de la force | La silhouette d'une île trahit son axe (5.4) |
| Nuit | Les raids restent visibles, mais leurs silhouettes sont moins lisibles. Un peuple aux Arcanes avancées voit mieux la nuit (3.6) |
| Pas de raid invisible | Sauf capacité Arcanes explicite, avec contre-mesure |

## 7.4 Pillage limité

| Règle 🔧 | Valeur |
|---|---|
| Plafond de pillage | **25 % maximum** des ressources stockées de la cible |
| Protection selon l'écart | Le plafond diminue si la cible est nettement plus faible |
| Bouclier après pillage | **15 min** de protection contre le même assaillant |
| Pas de destruction définitive | Les bâtiments sont **endommagés**, jamais supprimés, à l'exception du **cœur** (7.5) |

## 7.5 Le cœur et la chute

> **Une île ne chute que si son cœur est détruit.**

| Situation | Conséquence |
|---|---|
| **Défaite sans perte du cœur** | L'île entre en **état de « ruine »** : production réduite, reconstruction rapide. Le peuple ne perd ni sa culture ni son savoir |
| **Destruction du cœur** | L'île **chute** : elle est aspirée par le portail avec sa civilisation (1.3 bis) |

### Qui peut porter le coup fatal

| Menace | Peut détruire le cœur ? |
|---|---|
| **Monstres et boss** | Oui, à tout moment de la vie de l'île |
| **Joueurs et rivaux IA** | **Seulement par l'intention Anéantir, et seulement contre une île en déclin** |

> Les monstres usent la civilisation, les voisins l'achèvent : ce sont les barbares aux portes d'un empire qui s'effondre. Une île en pleine santé ne peut pas être rayée par une coalition de voisins.

### Le déclin

| Règle 🔧 | Détail |
|---|---|
| **Entrée en déclin** | Une île entre en déclin quand sa descente l'amène dans l'**anneau intérieur**, la zone la plus proche du portail (3.1) |
| **Présages** | La chronique annonce le déclin à l'avance (*« Les anciens disent que le tourbillon nous appelle. »*), et l'île change d'aspect : cœur fissuré, couleurs ternies |
| **Anéantir** | Très visible : bannière de guerre tournée vers la cible, bien plus marquée que celle de Convoiter. La cible et tout le ciel sont prévenus, la préparation est longue. Les voisins ont le temps de souffler Secourir |

Ce qui suit une chute est décrit en 11.5.

## 7.6 Raids et PvE

Pendant une vague PvE ou un boss (section 8), les raids PvP **restent possibles**. C'est volontaire : les joueurs peuvent coopérer ou profiter de l'occasion. Les assaillants prennent le risque de laisser leur propre île à découvert.

---

# 8. PvE : VAGUES ET BOSS

## 8.1 Principe

Des vagues de monstres attaquent les îles en continu. À intervalles réguliers, un **boss** menace l'ensemble du ciel.

## 8.2 Vagues ordinaires

| Aspect | Règle |
|---|---|
| Fréquence | Une vague toutes les 5 à 10 minutes 🔧 |
| Cible | Aléatoire, mais **pondérée par l'exposition** de l'île, c'est-à-dire sa proximité du portail (3.1) |
| Violence | Croît à mesure que l'île visée est proche du portail |
| Télégraphie | Un **indicateur visible** précède chaque vague de quelques dizaines de secondes |
| Nuit | Vagues plus fortes (+20 % 🔧), annoncées **dès le crépuscule** par l'indicateur de menace. Ce renfort reste sous le plafond de pression (8.3) |

## 8.3 PvE adaptatif : le système central

Le PvE module sa pression selon les entrées suivantes, **sans jamais surprendre**.

| Entrée | Description |
|---|---|
| **Nombre d'îles** | Îles vivantes du ciel, que leur joueur soit connecté ou non (les IA comptent à poids réduit, voir 9.5). Règle la pression **globale** du ciel. Se déconnecter ne change donc rien à la pression |
| **Puissance de l'île visée** | Une vague qui vise une île est calibrée sur la puissance (5.3) de **cette île**, jamais sur la moyenne du ciel. Un nouveau venu n'est pas écrasé par la force de ses voisins |
| **Distance au portail** | Plus l'île visée est proche du centre, plus la vague est violente (3.1) |
| **Puissance moyenne** | Sert uniquement à calibrer les **boss**, qui menacent tout le ciel (8.4) |

### Règles d'adaptation

| Règle | Détail |
|---|---|
| **Lissage** | La pression évolue **lentement** (fenêtre glissante), jamais par à-coups |
| **Plancher** | Toujours un minimum de menace, même seul |
| **Plafond par distance** | À une distance donnée du portail, la menace est bornée et prévisible. Elle ne croît sans fin qu'à mesure que l'île descend (1.3 bis) |
| **Hystérésis** | L'arrivée ou la chute d'une île ne change pas la pression d'un coup |
| **Lisibilité** | Un **indicateur de menace** affiche la pression courante et sa tendance |

### Principe anti-frustration

La difficulté s'ajuste **à l'état du ciel**, pas à la performance des joueurs. Un joueur qui joue mal ne reçoit pas de vagues plus faciles, un joueur qui joue bien n'en reçoit pas de plus dures. Seuls **le nombre d'îles, la puissance de l'île visée, sa distance au portail et l'heure du cycle** comptent. C'est ce qui garde le système honnête.

## 8.4 Les boss

| Aspect | Règle 🔧 |
|---|---|
| Fréquence | Toutes les 45 à 90 minutes |
| Cible | **Le ciel entier**, pas une île |
| Annonce | Longue, avec compte à rebours visible |
| Mécanique | Le boss sort du portail et parcourt le ciel en spirale, menaçant chaque île à son tour |
| Récompense | Partagée selon la **contribution** de chaque île : dégâts infligés au boss, y compris par les défenseurs envoyés par Secourir 🔧 |

## 8.5 Coopération et trahison

Face à un boss, le joueur choisit : **coopérer** (souffler **Secourir** vers un voisin menacé) ou **profiter** (raider les voisins occupés). Le système de récompense par contribution fait de la coopération un choix **rentable**, sans interdire la trahison.

## 8.6 Solo

Un joueur seul joue sur un serveur intégré (10.3) et affronte une pression calibrée **pour lui**. Le plancher de menace (8.3) et la présence des rivaux IA (section 9) garantissent une expérience complète sans autre humain.

---

# 9. RIVAUX IA

## 9.1 Rôle

Les rivaux IA peuplent le ciel pour que le jeu **soit vivant même en solo**, et servent de **banc de test** d'équilibrage automatisé.

## 9.2 Principe d'honnêteté

> **Les rivaux IA jouent avec les mêmes règles que les humains.**

Ils n'ont aucune information cachée, aucun bonus de production. Un rival IA est un **dieu IA** : son peuple est **exactement le même** que celui d'un joueur humain (6.4), et il n'agit qu'en soufflant les mêmes intentions, avec la même inertie (pilier 4). Le principe d'honnêteté est structurel, et il n'y a qu'un seul système de peuple à écrire.

## 9.3 Décision

Le dieu IA décide de son gâteau d'intentions, de son intention extérieure et de ses réponses aux suppliques. Son peuple fait le reste.

| Composant | Choix 🔧 |
|---|---|
| Architecture | **Machine à états + scores d'utilité**, au niveau du dieu (choix des intentions) comme du peuple (interprétation) |
| Cadence | Décision toutes les 5 à 15 secondes, pas à chaque frame |
| Aléa | Léger bruit sur les scores pour éviter la prévisibilité |

## 9.4 Profils de personnalité

| Profil | Comportement |
|---|---|
| 🗡️ **Belliqueux** | Souffle souvent S'armer et Punir, sa culture tend vers les Armes |
| 🛡️ **Prudent** | Souffle Se fortifier d'abord, convoite rarement |
| 🌾 **Bâtisseur** | Souffle Prospérer et S'étendre, évite le conflit |
| 🦊 **Opportuniste** | Convoite les îles occupées par un boss |

## 9.5 Intégration au système multi-joueurs

| Règle | Justification |
|---|---|
| Une IA compte à **poids réduit** dans la pression PvE | Évite l'emballement avec beaucoup d'IA |
| **Plafond de population** du ciel | 10 îles maximum, humains et IA confondus |
| **Un humain remplace une IA** quand le ciel est plein | La civilisation IA la plus ancienne abandonne son île (11.2), qui s'efface aussitôt pour libérer la place |
| Les IA se **signalent** comme telles | Transparence : un joueur sait toujours à qui il a affaire |

---

# 10. MULTIJOUEUR ET RÉSEAU

## 10.1 Découverte

Les clients découvrent les **serveurs** (10.3) par **mDNS** sur le réseau local, avec un **plan B** de connexion manuelle par IP, car beaucoup de réseaux d'entreprise bloquent le multicast.

| Cas | Mesure |
|---|---|
| **Android** | La réception multicast demande un verrou système (multicast lock) à prendre pendant la découverte |
| **Rejoindre depuis un téléphone** | Le serveur (ou le desktop qui l'héberge) affiche un **QR code** (adresse et port). Le téléphone le scanne pour rejoindre sans saisir d'IP 🔧 |
| **Wi-Fi isolant les appareils** | Beaucoup de Wi-Fi d'entreprise ou d'invités empêchent les appareils de se voir (isolation des clients). Aucun contournement possible côté jeu : la documentation pour les DSI doit le signaler |

## 10.2 Capacité

3 à 10 joueurs 🔧 (humains + IA), solo viable.

## 10.3 Modèle d'autorité : un serveur

> **Un serveur autoritaire unique** fait vivre le ciel. Les clients affichent, soufflent des intentions et reçoivent l'état.

| Choix | Description |
|---|---|
| **Serveur autoritaire** | Un programme sans rendu (`apogee-server`) exécute la simulation de référence : îles, peuples, PvE, orbites, cycle jour/nuit, rivaux IA. Il réutilise exactement le code de simulation du jeu |
| **Clients légers** | Les clients ne simulent pas : ils affichent l'état reçu, l'interpolent pour la fluidité et envoient les intentions du joueur |
| **Serveur dédié** | Pour un bureau : le serveur tourne en continu sur une machine toujours allumée (poste fixe, mini-PC, Raspberry Pi, machine virtuelle). C'est le mode recommandé |
| **Serveur intégré** | Le client desktop peut lancer le serveur dans son propre processus, pour jouer **en solo** ou improviser un ciel. Les autres le rejoignent tant que ce poste est allumé |
| **Android** | Ne fait jamais serveur : il peut être suspendu à tout moment |
| **Pas de migration** | Si le serveur s'arrête, le **ciel se fige** : plus rien ne bouge, ni production, ni vagues, ni âge. Tout reprend à son retour |
| **Persistance** | Le serveur sauvegarde le ciel entier (11.1) |

## 10.4 Cas à traiter

| Cas | Comportement |
|---|---|
| **Arrivée d'un joueur** | Snapshot complet de l'état du ciel, insertion progressive |
| **Départ d'un joueur** | Son île continue de vivre sur le serveur, son peuple livré à lui-même (pilier 3) |
| **Arrêt du serveur** | Le ciel se fige. Les clients se reconnectent automatiquement à son retour |
| **Déconnexion brève** | Reconnexion transparente avec resynchronisation |

> 📌 Une île de joueur absent **n'est pas protégée** (pilier 3). Elle produit, se défend, subit les vagues et les raids. Seul l'arrêt du serveur fige le ciel.

## 10.5 Principe de communication

On échange des **intentions, des graines et des états**, pas des assets. Les sprites sont générés localement à partir de graines partagées (section 12.2).

## 10.6 Sécurité et exposition

| Point | Mesure |
|---|---|
| Ports | Un seul port, configurable |
| Données | Aucune donnée personnelle échangée |
| Identité | Pas de compte : un pseudonyme et une **clé générée localement** par le client, qui permet au joueur de retrouver son île sur le serveur 🔧 |
| Réseau d'entreprise | Documentation claire pour les DSI, option de désactivation du mDNS |

---

# 11. PROGRESSION ET SESSIONS

## 11.1 Principes

| Principe | Application |
|---|---|
| **L'île vit sans son joueur** | Le serveur la fait vivre en continu, jeu ouvert ou fermé (pilier 3) |
| **Monde permanent** | Le ciel n'a ni saisons ni remise à zéro. Les civilisations, elles, naissent et chutent (11.2) |
| **Sauvegarde sur le serveur** | Le serveur sauvegarde le ciel entier : chaque île (bâtiments, peuple, culture, savoir), les orbites, les vestiges, les Annales. Le client ne détient rien de décisif, ce qui règle aussi la question de la triche |
| **Pas de récompense de connexion** | Aucune récompense de connexion quotidienne, aucune série à entretenir |

## 11.2 Monde permanent

> **Il n'y a pas de saisons.** Le ciel est permanent. Une île, son peuple, sa culture et son savoir persistent jusqu'à sa **chute** ou jusqu'à ce que son joueur l'**abandonne**.

| Règle | Détail |
|---|---|
| **Pas de remise à zéro collective** | Le ciel ne repart jamais de zéro. Une civilisation ne perd son savoir qu'en chutant (7.5) |
| **Abandon** | Seulement par un **choix explicite** du joueur. Une longue absence n'est pas un abandon : l'île continue de vivre, livrée à elle-même |
| **Devenir d'une île abandonnée** | Elle devient des **vestiges** que les monstres habitent quelque temps 🔧 (contenu PvE pour les voisins), puis s'efface et libère sa place dans le ciel |
| **Chute** | Une île peut aussi **chuter** quand son cœur est détruit (7.5) |
| **Renouvellement** | Il vient de la culture, qui peut toujours être réorientée, et du cycle des civilisations : chacune finit par chuter et laisser place à une autre (11.5) |

Conséquences :
- **L'écart entre anciens et nouveaux** n'est plus résorbé par les saisons : voir l'accueil des nouveaux venus (11.3).
- **Le long terme** : un peuple ne peut pas tout maîtriser, voir le plafond de savoir et l'oubli (5.3).
- **La fin** : toute civilisation finit par chuter (1.3 bis).

## 11.3 Accueil des nouveaux venus

Dans un monde permanent, un nouveau venu arrive dans un ciel d'îles avancées. Quatre mesures, cumulées :

| Mesure 🔧 | Détail |
|---|---|
| **Naissance en périphérie** | Sa civilisation apparaît au bord du ciel, loin du portail (3.1) |
| **Vagues à sa mesure** | Les vagues qui le visent sont calibrées sur sa puissance (8.3) |
| **Protection temporaire** | Aucun raid possible contre lui pendant ses premières heures d'existence (environ 12 h 🔧), signalée par un halo autour de l'île. Elle cesse plus tôt s'il souffle lui-même Convoiter ou Punir |
| **Rattrapage** | Production et savoir légèrement accrus tant que sa puissance est nettement sous la moyenne du ciel |

## 11.4 Rétention

Le jeu **ne cherche pas** la rétention par pression (pas de séries, pas de notifications culpabilisantes). La rétention vient de :
- la **curiosité** pour l'évolution de son île,
- la **dimension sociale** du bureau, nourrie par les Annales (11.5),
- le **projet de long terme** que représente une culture : réorienter son peuple vers un autre axe prend du temps.


## 11.5 Après la chute : renaissance et Annales

> **Une civilisation meurt, la lignée continue.**

| Règle 🔧 | Détail |
|---|---|
| **Renaissance** | Après sa chute, le joueur fonde une nouvelle civilisation sur une **nouvelle île**, avec une nouvelle graine. Elle émerge des nuages **en périphérie**, à l'aube suivante du cycle (3.6) |
| **Nouveau venu** | La nouvelle civilisation bénéficie de toutes les mesures d'accueil (11.3) |
| **Légende** | Le nouveau peuple garde la mémoire de ses ancêtres : sa culture de départ est légèrement teintée par la famille dominante de la civilisation précédente. **Aucun bonus de puissance**, pour ne pas désavantager les vrais nouveaux venus |
| **Annales du ciel** | Chaque civilisation y laisse une entrée, visible de tout le ciel : nom, joueur, durée, apogée (palier le plus haut, population), culture, cause de la chute (*« aspirée par le tourbillon sous les coups de l'île Rouge »*) |

Les Annales donnent sa mesure au talent du joueur : il ne gagne pas, il marque l'histoire du ciel par la durée et la hauteur de ses civilisations. Entre collègues, elles nourrissent la dimension sociale (*« la mienne a tenu six jours »*).

---

# 12. DIRECTION ARTISTIQUE ET TECHNIQUE VISUELLE

## 12.1 Style

**Pixel art à facettes, en 2D**, qui transpose en pixels l'esthétique du low-poly en flat shading (cel-shading) : volumes taillés en facettes planes, lumière unique, grands aplats, ombres portées colorées. Les pixels restent visibles.

| Principe | Application |
|---|---|
| **Volumes à facettes** | Rochers en prismes, arbres en cônes, falaises taillées : chaque forme est un petit nombre de facettes planes |
| **Trois tons par matière** | Face éclairée, face moyenne, face à l'ombre, sans dégradé |
| **Lumière unique et fixe** | Venue du haut à gauche 🔧, identique dans tout le jeu |
| **Ombres portées colorées** | Longues, teintées de bleu-violet, jamais noires |
| **Pas de contour** | Les bords naissent du contraste de valeur entre facettes |
| **Aplats calmes** | Le sol est une zone de repos visuel, le détail se concentre sur les objets |
| **Couleur qui code le sens** | Nature désaturée, **saturation réservée** aux bâtiments, aux équipes et aux alertes |
| **Vue 3/4 plongeante** | Projection oblique d'environ 60° 🔧, pas d'isométrie stricte |

## 12.2 Génération procédurale des assets

Tous les sprites sont **générés par code**, par rastérisation de facettes (paquet `internal/art`).

```
Gabarit (petit mesh 3D à facettes) → Variation (graine) → Projection oblique 3/4
  → Ton par facette (normale · lumière → rampe) → Rastérisation sans anticrénelage
  → Silhouette d'ombre projetée au sol → Atlas en cache
```

| Règle | Détail |
|---|---|
| **Gabarits à facettes** | Un gabarit est un **petit mesh 3D** de quelques facettes planes (cônes, prismes, boîtes, toits). Il est projeté en vue oblique, les faces cachées sont éliminées et les autres peintes de l'arrière vers l'avant |
| **Palette en rampes** | Environ 8 matières × 3 tons 🔧, plus une couleur d'ombre, plus les couleurs d'équipe |
| **Ton par facette** | Choisi dans la rampe de la matière selon le produit scalaire entre la normale de la facette et la direction de la lumière fixe : dessus et faces ouest éclairés, faces sud moyennes, faces est sombres |
| **Variations par graine** | Hauteur, largeur, nombre de facettes (un arbre est un cône aux proportions tirées au sort) |
| **Ombre portée** | Chaque sprite produit aussi sa silhouette d'ombre, projetée dans la direction de la lumière |
| **Contour** | Aucun sur le décor et les bâtiments. Un contour léger **sur les unités seulement**, pour la lisibilité au dézoom |
| **Orientations** | Le mesh est tourné **avant** le calcul des tons : une rotation ne fausse donc jamais la lumière. La vertical slice utilise 4 orientations (quarts de tour) tirées par la graine ; d'autres angles restent possibles sans changer le pipeline 🔧 |
| **Grilles de sprites** | Sprites de 8×8 à 32×32 pixels virtuels |
| **Gabarits conçus à la main** | Les formes de base sont écrites à la main, la graine ne fait que les varier, pour éviter l'aspect « sans âme » du pur procédural |
| **Graine par île** | Terrain et décors reproductibles sans transfert d'images |
| **Cache** | Génération au premier lancement, stockage disque |

> 💡 **Choix d'implémentation** : la version initiale du GDD prévoyait des polygones 2D étiquetés à la main. Le spike de Phase 0 a montré qu'un mini-rastériseur 3D orthographique tient en quelques centaines de lignes et donne un éclairage exact. C'est donc l'approche retenue.

## 12.3 Rendu

| Aspect | Choix |
|---|---|
| **Résolution virtuelle basse** | Rendu sur petite grille puis agrandissement « plus proche voisin » |
| **Ordre d'affichage** | Tri des objets par le y de leur base à chaque frame (vue 3/4, placement libre) |
| **Calque d'ombres** | Toutes les ombres sont dessinées d'une seule couleur dans un calque séparé, composé une seule fois avec une opacité uniforme : deux ombres qui se chevauchent ne s'assombrissent pas |
| **Positions arrondies** | Arrondies au pixel virtuel au moment du dessin, pour éviter le tremblement des sprites |
| **Niveaux de détail** | Simplification progressive en dézoomant |
| **Lisibilité d'abord** | Une unité reste identifiable à toutes les échelles |

> **Zoom par paliers entiers.** Un agrandissement d'un facteur non entier donne des pixels de tailles inégales qui scintillent au déplacement. Le zoom se fait donc par facteurs entiers (×1, ×2, ×3…), un par vue (« Ciel », « Île », « Détail »), avec une **transition animée** entre deux paliers pour garder une impression de continuité.

## 12.4 Lisibilité

- **Couleur d'équipe** forte et cohérente (accessibilité daltonisme prévue)
- **Axes technologiques** reconnaissables par silhouette
- **Indicateurs** d'alerte et de menace toujours visibles
- **Mode discret** : fenêtre épurée pour usage au bureau

## 12.5 Jour et nuit

L'éclairage des facettes est calculé dans les sprites : on ne fait pas tourner le soleil. Le cycle jour/nuit (3.6) passe par des moyens peu coûteux :

| Technique | Détail |
|---|---|
| **Teinte de palette par moment** | Chaque moment (aube, jour, crépuscule, nuit) a sa version des rampes : décalée vers le bleu et moins contrastée la nuit, dorée à l'aube et au crépuscule. Par atlas précalculés en cache ou par table de correspondance de couleurs dans un shader 🔧 |
| **Ombres mobiles** | Le calque d'ombres (12.3) est incliné au moment du dessin : les ombres s'allongent et tournent au fil de la journée, sans rien régénérer |
| **Lumières de nuit** | Fenêtres, torches et cristaux d'Aether en halos additifs |
| **Transitions lentes** | Interpolation lente entre deux moments, compatible avec le budget CPU en arrière-plan (13.3) |
| **Lisibilité** | La nuit reste assez claire pour se lire d'un coup d'œil sur un second écran |

## 12.6 Les îles dans le ciel

Le ciel offre des ressources visuelles que les jeux au sol n'ont pas :

| Élément | Rôle |
|---|---|
| **Le portail** | Un **tourbillon géant** au centre du ciel, point focal de la vue « Ciel ». Il s'agite avant chaque vague et chaque boss |
| **L'aspiration** | Une île qui chute est entraînée en spirale dans le tourbillon, se disloque et disparaît. C'est le moment le plus spectaculaire du jeu, visible de tout le ciel |
| **Orbites** | En vue 3/4, les orbites sont des ellipses. La rotation est lente, et la caméra des vues « Île » et « Détail » suit l'île du joueur |
| **Dessous de l'île** | Un cône rocheux à facettes, éclairé d'un côté, qui donne une silhouette forte en vue « Ciel » |
| **Profondeur atmosphérique** | Les îles lointaines ou basses sont plus bleutées et moins contrastées, sans coût de rendu |
| **Ombres sur les nuages** | L'ombre d'une île tombe sur une couche de nuages en contrebas, ce qui situe les îles les unes par rapport aux autres |

## 12.7 Audio

| Aspect | Choix |
|---|---|
| Style | Chiptune sobre, bruitages discrets |
| Philosophie | **Muet par défaut**, sons activables, jamais envahissants |
| Alertes | Un son bref et clair pour une attaque entrante |

---

# 13. INTERFACE ET EXPÉRIENCE

## 13.1 Principes

| Principe | Application |
|---|---|
| **Compatible tactile** | Dès le MVP Linux : aucune information réservée au survol, glisser pour ajuster le gâteau d'intentions. À la phase Android : cibles d'au moins 48 dp, pincement pour zoomer |
| **Écrans variés** | La résolution virtuelle s'adapte au format de l'écran, sans déformer les pixels (12.3) |
| **Défilement** | Quand la fenêtre est trop petite pour toute la scène, les **flèches du clavier** la font défiler, sans jamais sortir de ses limites. Sur un axe où la scène tient, elle reste centrée |
| **Orientation** | Non décidée sur Android, voir annexe A, décision n° 17 (à trancher en phase 5) |
| **Discrétion** | Peut tourner dans une fenêtre réduite (desktop) ou sur un téléphone posé à côté |
| **Lecture rapide** | L'état de l'île se saisit en un coup d'œil |
| **Notifications système** | Les alertes de raid et les messages de la chronique peuvent s'afficher en **notifications du bureau**, pour suivre son île quand la fenêtre est cachée. Le joueur choisit lesquelles recevoir (raids, vagues, déclin, suppliques, tout) 🔧 |
| **Pas de pop-ups dans le jeu** | Aucune fenêtre qui interrompt la scène : alertes et chronique s'intègrent à l'interface |
| **UI dans le style du jeu** | Dessinée en pixel art, cohérente |

## 13.1 bis Une île vivante

| Élément | Application |
|---|---|
| **Croissance visible** | Une île naissante ne porte que son cœur et ses sites naturels. Le peuple mène jusqu'à **deux chantiers à la fois**, longs (de 30 min à plus d'une heure chacun), et la natalité est lente : l'île se remplit en une quinzaine d'heures 🔧 |
| **Villageois** | Ils vont de leur maison aux chantiers, aux champs, aux carrières et aux bosquets, y travaillent un moment, puis rentrent. La nuit, ils restent davantage chez eux. Ils passent derrière ou devant les bâtiments selon leur profondeur |
| **Défenseurs** | Ils patrouillent autour des tours et du cœur, sans jamais quitter l'île |
| **Particules** | Feuilles qui tombent des bosquets, sciure sur les chantiers, éclats aux carrières, balle dorée sur les champs, étincelles d'Aether |
| **Décor seulement** | Ces animations sont calculées par le client : la simulation ne connaît que les effectifs et les bâtiments |

## 13.1 ter Accueil et tutoriel

| Élément | Application |
|---|---|
| **Écran d'accueil** | Nom du dieu, continuer ou recommencer la partie solo, héberger un ciel sur le réseau (annoncé en mDNS), rejoindre un ciel découvert ou une adresse, quitter |
| **Tutoriel** | Au premier lancement : dix étapes courtes, chacune encadrant l'élément dont elle parle (souffles, conviction, chronique, suppliques, ressources, ciel, autres îles, grandeur et chute). On peut le passer, le rouvrir avec H, ou le réactiver depuis l'accueil |

## 13.2 Éléments d'écran

| Élément | Contenu |
|---|---|
| **Gâteau d'intentions** | Les trois parts courantes, avec la cible et la conviction réelle du peuple |
| **Intention extérieure** | L'intention en cours et son île cible, Rappeler toujours accessible |
| **Supplique** | La supplique en attente, ses réponses et le temps avant que le peuple tranche seul |
| **Chronique** | Les derniers événements, en quelques lignes |
| **Culture et savoir** | Le profil de culture par famille, l'axe suivi par les savants et les découvertes acquises. **Consultation seule** |
| **Panneau ressources** | Quatre ressources avec plafonds visibles |
| **Indicateur de menace** | Pression PvE courante et tendance, y compris l'annonce de la nuit |
| **Moment de la journée** | Position dans le cycle jour/nuit et temps avant le prochain moment |
| **Liste des îles** | Voisins, leur axe dominant, statut (dont le déclin) |
| **Annales du ciel** | Les civilisations passées et présentes, leur durée, leur apogée, leur chute (11.5) |
| **Alerte de raid** | Notification claire quand une attaque approche, dans le jeu et en notification système |

## 13.3 Performances en arrière-plan

| Mode | Comportement |
|---|---|
| **Actif** | Rendu complet |
| **Arrière-plan** | Rendu suspendu. Le client ne simulant rien (10.3), il ne fait plus que recevoir l'état : c'est ce qui rend l'objectif de CPU quasi nul atteignable |
| **Client inactif** | Fréquence minimale |
| **Android en arrière-plan** | Jeu suspendu par le système (`suspendGame` / `resumeGame` d'Ebitengine). L'île continue de vivre sur le serveur |
| **Android au premier plan** | Fréquence d'images plafonnée et rendu minimal quand rien ne bouge, pour la **batterie** et la chauffe |

> 🔧 L'objectif est un **CPU quasi nul** en arrière-plan sur desktop, et une consommation compatible avec un téléphone allumé plusieurs heures. À valider dès la Phase 0.

---

# 14. ARCHITECTURE TECHNIQUE

## 14.1 Stack

| Composant | Choix |
|---|---|
| Langage | **Go** |
| Moteur de rendu | **Ebitengine** |
| Découverte | `hashicorp/mdns` ou `grandcat/zeroconf` |
| Distribution desktop | Binaire unique. **Linux d'abord**, Windows et macOS en phase 5 par compilation croisée |
| Notifications | Sous Linux, notifications du bureau via le standard freedesktop (D-Bus). Les autres plateformes en phase 5 |
| Distribution Android (phase 5) | `ebitenmobile bind` produit une bibliothèque `.aar` (vue `EbitenView`) intégrée à un petit projet Android Studio |

## 14.2 Découpage

```
        SERVEUR (apogee-server, sans rendu) 
┌───────────────────────────────────────────────┐
│  Simulation (pas fixe)   ◄──  IA des peuples  │
│  PvE, orbites, cycle          et des dieux IA │
│  Persistance du ciel                          │
└───────────────────────┬───────────────────────┘
                        │ état ▼   ▲ intentions
        ┌───────────────┴───────────────┐
┌───────┴────────┐              ┌───────┴────────┐
│ CLIENT Linux   │     ...      │ CLIENT Android │
│ Rendu Ebiten   │              │ Rendu Ebiten   │
│ Interface      │              │ Interface      │
└────────────────┘              └────────────────┘

Le paquet de simulation est partagé : le serveur l'exécute,
le serveur intégré l'embarque dans le client desktop.
```

## 14.3 Règles d'architecture

| Règle | Justification |
|---|---|
| **Simulation à pas fixe** découplée du rendu | Stabilité, testabilité |
| **Structures plates et pools** | Éviter les pauses du ramasse-miettes |
| **Pas d'itération sur `map` dans la simulation** | L'ordre est aléatoire en Go |
| **Coordonnées locales à l'île** | Bâtiments et unités vivent dans le repère de leur île, qui se déplace en orbite. Seuls les raids, les vagues et les boss utilisent le repère du ciel |
| **Grille d'occupation cachée** | Le placement est libre à l'écran (3.5), mais une grille fine invisible sert aux tests d'occupation et à la recherche de chemin, plus simple qu'un navmesh |
| **Simulation déterministe sur une machine** | Générateur aléatoire à graine, ordre d'itération stable : tests et rejeu facilités. Pas besoin de virgule fixe, puisque seul le serveur simule (10.3) |
| **Profilage dès le prototype** | `pprof` intégré tôt |

## 14.4 Tests

| Type | Usage |
|---|---|
| **Simulation sans rendu** | Parties IA contre IA accélérées |
| **Équilibrage automatisé** | Des milliers de parties pour détecter les stratégies dominantes |
| **Tests longue durée** | Fuites mémoire sur 8 h |
| **Tests réseau réels** | Machines physiques, vrai réseau d'entreprise. Téléphones Android sur le Wi-Fi en phase 5 |
| **Tests Android** (phase 5) | Plusieurs appareils réels, dont un d'entrée de gamme : performances, batterie, chauffe, suspension et reprise. Vérifier comment recevoir les notifications de la chronique quand l'application est suspendue |

---

# 15. PLAN DE PRODUCTION

*(Hypothèse : développeur senior solo.)*

| Phase | Contenu | Critère de sortie |
|---|---|---|
| **0. Pré-production** | GDD, charte graphique, spikes serveur et mDNS, rendu/CPU sous Linux | Les deux spikes valident la faisabilité |
| **1. Vertical slice** | Serveur intégré (sans réseau), 1 île, peuple et intentions, suppliques, culture, chronique, 4 axes à 2 paliers, orbite et descente, chute et renaissance, 1 vague, 1 boss, 1 rival IA, sprites générés | *« A-t-on envie de laisser ça tourner ? »* |
| **2. Alpha** | Serveur dédié et réseau, PvP, plusieurs IA, PvE adaptatif v1, cycle jour/nuit, Annales | 3 machines, 8 h sans perte de connexion ni état incohérent |
| **3. Bêta** | 3e palier technologique (Excellence), équilibrage automatisé, sauvegarde robuste (versions, migration), tests longs | 8 h de stabilité, tests humains |
| **4. Lancement** | Distribution, contenus, boucle de retours | Binaire Linux stable |
| **5. Autres plateformes** | **Android** : spike (build, découverte sur Wi-Fi, batterie), portage, interface tactile, notifications, tests sur appareils. **Windows et macOS** : compilation croisée, notifications, tests | Un téléphone, un poste Windows et un Mac jouent 8 h dans le même ciel |

## 15.1 Résultats de la Phase 0

| Spike | Résultat | Mesure |
|---|---|---|
| **Rendu à facettes** | ✅ Validé, direction artistique approuvée | Générateur en Go pur (`internal/art`), sans dépendance, déterministe et testé. Aperçu sans fenêtre : `go run ./cmd/artpreview`. Différé à la vertical slice : couleur d'équipe par joueur (les unités utilisent une couleur unique), contour léger des unités, orientations des bâtiments autres que les maisons |
| **CPU du client** | ✅ Encourageant | Fenêtre ouverte, nuages en mouvement : **environ 2 % d'un cœur** sous Linux, en ne redessinant que lorsque l'image change. Reste à mesurer fenêtre sans focus et minimisée |
| **Serveur et mDNS** | ✅ Validé sur le réseau de développement | `apogee-server` annoncé en mDNS, découvert et rejoint par `apogee-probe`, états reçus. Reste à tester sur un vrai réseau d'entreprise (risque n° 3) |

## 15.2 État des Phases 1 et 2

| Élément | État | Notes |
|---|---|---|
| **Peuple et intentions** | ✅ | Gâteau de 3 parts au plus, conviction à inertie (environ 90 s), interprétation par scores d'utilité, placement libre sur le contour partagé (`internal/geo`). La survie (fermes, maisons) passe avant les souhaits du dieu |
| **Culture et technologie** | ✅ | 4 axes à 2 paliers, savoir tiré de l'Aether, plafond de 3 paliers et oubli. Paliers à 300 puis 900 points 🔧 : premier palier vers 8 h pour un peuple prospère, en 1 à 2 h pour un peuple studieux |
| **Suppliques et chronique** | ✅ | Découvertes, manque de place, famine. Un refus de s'agrandir espace les demandes (2 h, puis le double à chaque refus). La chronique annonce chaque dizaine d'âmes une seule fois |
| **Cycle de vie** | ✅ | Descente par l'âge et la puissance, déclin, vagues inexorables avec l'âge, colosse, ruine, chute, renaissance à l'aube suivante, Annales. Durées de vie observées en simulation : 2 à 4 jours selon le gâteau |
| **Rivaux IA** | ✅ | 4 profils, mêmes règles que les humains, ils répondent aux suppliques et lancent des raids (belliqueux, opportuniste) |
| **Serveur** | ✅ | Autoritaire, intégré ou dédié, sauvegarde atomique du ciel, accélération du temps pour les tests (`-speed`), plafond de 10 îles |
| **PvP** | ✅ | Convoiter, Punir, Anéantir, Secourir, Rappeler. Pillage limité à 25 % et à la charge des guerriers, bouclier de 15 min, protection des nouveaux venus pendant 12 h sauf s'ils attaquent eux-mêmes |
| **Interface** | ✅ | Vues « Ciel » et « Île » pilotées par l'état du serveur, HUD en pixel art (police bitmap, typographie française), panneau de cible, bannières, raids et vagues visibles, teinte jour/nuit |
| **Notifications** | ✅ | `notify-send` sous Linux, catégories au choix (`-notify`), seulement fenêtre sans focus |
| **Critère de sortie de la Phase 1** | ⏳ | « A-t-on envie de laisser ça tourner ? » : à juger par une partie réelle |
| **Critère de sortie de la Phase 2** | ⏳ | 3 machines, 8 h : à faire sur un vrai réseau. Un test d'endurance local est passé : 3 clients, temps ×200, environ 16 h de jeu, 870 états reçus, aucune erreur, 18 Mo de mémoire côté serveur, sauvegarde propre à l'arrêt |
| **Île vivante, accueil, tutoriel, observation** | ✅ | Voir 13.1 bis et ter, 3.4 |
| **Reporté** | | Couleur d'équipe par joueur, contour des unités, lumières de nuit (halos), ombres mobiles |

## 15.3 Priorités MVP (MoSCoW)

| Priorité | Éléments |
|---|---|
| **Must** | Simulation, sauvegarde, rendu pixel art généré, zoom, peuple et gâteau d'intentions, suppliques, culture, chronique, 4 axes tech à 2 paliers, orbite autour du portail, descente, chute et renaissance, PvE de base, 1 rival IA, serveur (intégré et dédié) et découverte mDNS, boucle idle, build Linux |
| **Should** | Raids PvP (Convoiter, Punir, Anéantir), Secourir, PvE adaptatif, protection des nouveaux, indicateur de menace, cycle jour/nuit, Annales, notifications système |
| **Could** | 3e palier technologique, profils IA variés, effets poussés, Android, Windows et macOS (phase 5) |
| **Won't** | Chat, classements globaux, boutique, serveur public sur Internet, jeu hors LAN |

---

# 16. RISQUES

| # | Risque | Gravité | Mitigation |
|---|---|---|---|
| 1 | La boucle idle est ennuyeuse | 🔴 Critique | Test d'âme en Phase 1, avant tout réseau |
| 2 | CPU trop élevé en arrière-plan | 🔴 Critique | Spike en Phase 0, mode économie |
| 3 | Réseaux d'entreprise bloquant mDNS | 🟡 Élevé | Connexion manuelle, test sur vrai réseau pro |
| 4 | Désynchronisation | 🟢 Faible | Serveur autoritaire unique, clients sans simulation, snapshots |
| 5 | PvE adaptatif frustrant | 🟡 Élevé | Lissage, indicateur, présages du déclin, tests IA massifs |
| 6 | Dérive de périmètre du solo | 🟡 Élevé | MoSCoW, critères de sortie stricts |
| 7 | Sprites procéduraux sans âme | 🟡 Moyen | Gabarits à facettes conçus à la main, palette en rampes, lumière unique |
| 8 | Exploitation du système adaptatif | 🟢 Faible | La pression compte les îles vivantes : se déconnecter ne change rien (8.3) |
| 9 | Îles désordonnées et illisibles en placement libre | 🟡 Moyen | Marge minimale entre emprises, aplats calmes au sol, test de lisibilité en vue « Ciel » |
| 10 | Le joueur se sent impuissant (« mon peuple ne m'écoute pas ») | 🔴 Critique | Inertie courte, chronique, signes visibles dans la scène, test dès la Phase 1 |
| 11 | Les suppliques deviennent une corvée | 🟡 Moyen | Rares, facultatives, sans accumulation ni pénalité |
| 12 | Écart entre anciens et nouveaux dans un monde permanent | 🟡 Élevé | Vagues calibrées par île, protection temporaire, rattrapage (11.3) |
| 13 | Acharnement de plusieurs joueurs sur le cœur d'une même île | 🟡 Élevé | Coup fatal réservé à Anéantir contre une île en déclin, très visible et lent à préparer (7.5) |
| 14 | La chute vécue comme une punition plutôt que comme une fin | 🟡 Élevé | Chute annoncée (pilier 5), spectacle de l'aspiration, Annales, renaissance immédiate |
| 15 | Wi-Fi d'entreprise isolant les téléphones des desktops | 🟡 Élevé | Spike en début de phase 5, QR code, connexion manuelle par IP, documentation DSI |
| 16 | Batterie et chauffe sur téléphone | 🟡 Élevé | Fréquence plafonnée, rendu minimal au repos, tests sur appareil d'entrée de gamme |
| 17 | Charge de travail de plusieurs plateformes pour un développeur solo | 🟡 Moyen | Plateformes l'une après l'autre (Linux, puis Android, Windows et macOS), une seule base de code, interface compatible tactile dès le départ |
| 18 | Personne ne fait tourner de serveur au bureau | 🟡 Moyen | Serveur intégré au client desktop, binaire serveur léger et documenté |
| 19 | Chute d'une île pendant une absence longue (week-end, vacances) | 🟡 Accepté | Risque assumé (1.3 bis). Résumé de la chronique au retour, Annales, renaissance immédiate |

---

# ANNEXE A. JOURNAL DES DÉCISIONS

Les décisions marquées ✅ sont tranchées et intégrées au corps du document. Les autres restent ouvertes, avec une recommandation.

| # | Décision | Options | Recommandation |
|---|---|---|---|
| **1** | ~~Saisons ou monde permanent ?~~ | ✅ **Tranché** : monde permanent, tant que le joueur n'abandonne pas son île (11.2) | |
| **2** | ~~Ordre « Renfort » ?~~ | ✅ **Tranché** : devient l'intention **Secourir** (6.3) | |
| **3** | ~~Combien d'axes tech au MVP ?~~ | ✅ **Tranché** : 4 axes à 2 paliers au MVP, 3 paliers en bêta (5.3) | |
| **4** | ~~Anti-exploitation du PvE adaptatif ?~~ | ✅ **Tranché** : se déconnecter ne change rien, la pression compte les îles et non les connexions (8.3) | |
| **5** | ~~Qu'est-ce qui est « sauvegardé » ?~~ | ✅ **Tranché** : l'île complète, imposé par le monde permanent (11.1) | |
| **6** | ~~Compte-t-on les joueurs par île ou par session ?~~ | ✅ **Tranché** : par île vivante (8.3) | |
| **7** | ~~Nom définitif ?~~ | ✅ **Tranché** : **Apogée**, le sommet juste avant la chute. Disponibilité à vérifier (jeux existants, marques, noms de domaine). Identifiants techniques sans accent : `apogee` | |
| **8** | ~~Qui décide *quoi* construire ?~~ | ✅ **Tranché** : le peuple, à partir des intentions (section 6) | |
| **9** | ~~Orientations des bâtiments ?~~ | ✅ **Tranché** : rotation avant le calcul des tons, donc sans fausser la lumière ; 4 orientations dans la vertical slice (12.2) | |
| **10** | ~~Mode de zoom ?~~ | ✅ **Tranché** : paliers entiers avec transition animée (3.4, 12.3) | |
| **11** | ~~Cycle jour/nuit ?~~ | ✅ **Tranché** : rythme léger, cycle de 30 min commun au ciel (3.6) | |
| **12** | ~~Qu'est-ce qu'un abandon ?~~ | ✅ **Tranché** : abandon explicite seulement, l'île devient des vestiges puis s'efface (11.2) | |
| **13** | ~~Accueillir un nouveau venu ?~~ | ✅ **Tranché** : vagues calibrées par île, protection temporaire, rattrapage (8.3, 11.3) | |
| **14** | ~~Que fait un peuple qui a tout atteint ?~~ | ✅ **Tranché** : plafond de savoir et oubli (5.3) | |
| **15** | ~~Grandeur et chute : la suite ?~~ | ✅ **Tranché** : une semaine de temps réel au plus (1.3 bis), renaissance, légende et Annales (11.5), pilier 5 réécrit, coup fatal par Anéantir (7.5) | |
| **16** | ~~Ce qui rapproche une île du portail ?~~ | ✅ **Tranché** : l'âge et la puissance (3.1) | |
| **17** | **Orientation de l'écran sur Android** | Paysage seulement / portrait seulement / les deux | 🟡 Paysage seulement au MVP : même mise en page que le desktop, une seule interface à concevoir. Le portrait, plus naturel pour un téléphone posé sur un support, viendra plus tard s'il est demandé |
| **18** | ~~Pairs ou serveur ?~~ | ✅ **Tranché** : serveur autoritaire, dédié ou intégré au client desktop (10.3) | |
| **19** | ~~Les absences longues : nuits, week-ends, vacances ?~~ | ✅ **Tranché** : aucun garde-fou, le peuple se débrouille (1.3 bis) | |

---

# ANNEXE B. GLOSSAIRE

| Terme | Définition |
|---|---|
| **Annales du ciel** | Registre des civilisations passées et présentes, visible de tout le ciel |
| **Axe** | Une des quatre voies technologiques |
| **Chronique** | Récit court de ce que le peuple a fait et pourquoi |
| **Chute** | Fin d'une civilisation : son cœur détruit, l'île est aspirée par le portail |
| **Ciel figé** | État du ciel quand le serveur est arrêté : plus rien n'avance |
| **Conviction** | Degré d'adhésion du peuple à une intention, qui glisse lentement vers la part soufflée |
| **Culture** | Mémoire lente des intentions soufflées, par famille. Elle donne des bonus et oriente la technologie |
| **Cœur** | Bâtiment central qui maintient l'île dans le ciel. Sa destruction provoque la chute |
| **Cycle jour/nuit** | Alternance de 30 min commune à tout le ciel, qui module légèrement production, savoir et menace |
| **Déclin** | Dernière phase d'une civilisation, dans l'anneau intérieur près du portail. Seule une île en déclin peut être visée par Anéantir |
| **Emprise** | Forme simple qu'occupe un bâtiment au sol, utilisée pour valider un placement |
| **Facette** | Polygone plan d'un sprite, coloré d'un seul ton selon son orientation face à la lumière |
| **Famille** | Regroupement d'intentions correspondant à un axe technologique |
| **Gâteau** | Répartition d'au plus trois intentions tournées vers l'île |
| **Graine** | Valeur qui permet de régénérer un contenu à l'identique |
| **Île abandonnée** | Île à laquelle son joueur a renoncé. Elle devient des **vestiges**, puis s'efface |
| **Intention** | Volonté soufflée par le dieu, que le peuple interprète |
| **Légende** | Mémoire des ancêtres qui teinte la culture de départ d'une nouvelle civilisation |
| **Orbite** | Trajectoire d'une île autour du portail. Sa distance au centre fixe son exposition |
| **Oubli** | Perte progressive des paliers d'un axe délaissé quand le plafond de savoir est atteint |
| **Palier** | Un niveau de progression dans un axe |
| **Portail** | Tourbillon géant au centre du ciel, d'où surgissent monstres et boss, et qui aspire les îles qui chutent |
| **Pression PvE** | Intensité calculée des vagues |
| **Puissance** | Mesure composite d'une île (paliers, bâtiments, population), qui règle sa descente, le pillage et le calibrage des vagues |
| **Rampe** | Les trois tons (éclairé, moyen, ombre) d'une même matière dans la palette |
| **Renaissance** | Fondation d'une nouvelle civilisation après une chute |
| **Serveur autoritaire** | Programme sans rendu qui exécute la simulation de référence du ciel et le sauvegarde |
| **Serveur dédié** | Serveur qui tourne en continu sur une machine toujours allumée |
| **Serveur intégré** | Serveur lancé dans le processus du client desktop, pour le solo ou un ciel improvisé |
| **Supplique** | Question du peuple à son dieu, tranchée par le peuple s'il n'y a pas de réponse |
| **Vestiges** | Restes d'une île abandonnée, habités un temps par les monstres |
