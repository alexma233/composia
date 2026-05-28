---
title: "Secrets"
date: '2026-05-26T00:00:00+08:00'
weight: 50
---

Composia verwaltet verschlüsselte Secret-Dateien im Sollzustand-Repository mit Age-Verschlüsselung. Ver- und Entschlüsselung finden auf dem Controller statt. Agenten erhalten niemals Zugriff auf den privaten Age-Schlüssel.

## Konfiguration

Secrets erfordern ein Age-Schlüsselpaar. In der Controller-Konfiguration einrichten:

```yaml
controller:
  secrets:
    provider: age
    identity_file: "/app/configs/age-identity.key"
```

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `provider` | `string` | Ja | Muss `age` sein. |
| `identity_file` | `string` | Ja | Pfad zur privaten Age-Schlüsseldatei. |
| `recipient_file` | `string` | Nein | Pfad zur Datei mit Age-Empfängern (öffentliche Schlüssel). Wenn weggelassen, wird der Empfänger vom privaten Schlüssel abgeleitet. |
| `armor` | `bool` | Nein | Verwendet ASCII-armierte Ausgabe. Standardmäßig `true`. |

Generiere ein Schlüsselpaar:

```bash
age-keygen -o age-identity.key
```

Optional: Extrahiere den öffentlichen Schlüssel als Empfänger:

```bash
age-keygen -y age-identity.key > age-recipients.txt
```

## Wie Secrets gespeichert werden

Secret-Dateien im Repository haben konventionsgemäß die Erweiterung `.enc`. Sie werden als Age-verschlüsselter Geheimtext gespeichert:

```
my-app/
├── docker-compose.yaml
├── composia-meta.yaml
└── .secret.env.enc        (mit Age verschlüsselt)
```

Der Controller verschlüsselt Klartext beim Schreiben und entschlüsselt beim Lesen. Das Repository enthält nur Geheimtext. Secrets erscheinen niemals als Klartext im Repo, in Aufgabenprotokollen oder bei der Übertragung an Agenten.

## Wie Secrets zu Agenten gelangen

Während des Render-Schritts einer Deploy- oder Update-Aufgabe:

1. Liest der Controller verschlüsselte Dateien aus dem Dienstverzeichnis im Repo.
2. Entschlüsselt jede Datei mit dem privaten Age-Schlüssel.
3. Injiziert den entschlüsselten Inhalt als `.composia-secret.env` in das Dienstpaket.

Das Paket wird über die Agent-Report-Verbindung an den Agenten gestreamt. Der Agent schreibt das Paket auf die Festplatte und fährt mit `docker compose up` fort. Die entschlüsselte Secret-Umgebung steht den Compose-Diensten zur Verfügung, ohne dass der Agent jemals den privaten Schlüssel sieht.

## CLI-Nutzung

Schreibe eine verschlüsselte Secret-Datei:

```bash
composia secret update my-app .secret.env.enc --file ./local-plain.env
```

Lese und entschlüssele eine Secret-Datei:

```bash
composia secret get my-app .secret.env.enc
```

Bearbeite ein Secret direkt (öffnet deinen Editor):

```bash
composia secret edit my-app .secret.env.enc
```

Alle Secret-Schreiboperationen beinhalten eine Basis-Revisionsprüfung, um Konflikte mit gleichzeitigen Änderungen zu verhindern.

## Dateipfad-Regeln

Secret-Dateipfade müssen:

- Relativ zum Dienstverzeichnis sein (nicht absolut).
- Keine Pfadtraversierungssequenzen wie `../` enthalten.
- Auf eine Datei innerhalb des Dienstverzeichnisses verweisen.

Der Controller lokalisiert den Dienst, löst den Dateipfad relativ zum Dienstverzeichnis auf und operiert auf der Repo-Datei.

## Fehlerbedingungen

- **Secrets nicht konfiguriert**: `GetSecret` und `UpdateSecret` geben `FailedPrecondition` zurück, wenn `controller.secrets` nicht gesetzt ist.
- **Datei nicht gefunden**: `GetSecret` gibt eine leere Inhaltsantwort statt eines Fehlers zurück, wenn die Datei nicht existiert. Dies ermöglicht Clients, zwischen fehlenden Dateien und Entschlüsselungsfehlern zu unterscheiden.
- **Basis-Revisionskonflikt**: `UpdateSecret` verwendet CAS (Compare-and-Swap) gegen den Repo-HEAD. Wenn sich das Repo seit dem letzten Lesen geändert hat, schlägt das Schreiben mit einem Revisionskonflikt fehl.
