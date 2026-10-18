# Task07a: what should be simpler?

I reviewed the backup changes through commit cc998db. This was initially a read-only review. The user subsequently approved fixed runtime
and backup working filenames, followed by removal of Linux mount inspection.
Items 3, 4 and 5 remain proposals.
This review covers task07a, not the earlier tasks. My own plan is not proof that
all the code it called for was necessary.

## Current status

| Item | Status |
| --- | --- |
| 1. Fixed running-database location | Done previously |
| 2. Remove Linux mount inspection | Done in this update |
| 3. Remove the redundant backup lock | Not done |
| 4. Remove the half-built Store | Not done |
| 5. Improve backup failure messages | Not done |

This update does item 2 only. It does not implement items 3, 4 or 5.

## What I would change

### 1. Put the running database in the same place every time

Today, every startup creates a randomly named folder for the running database.
The app then needs code to find and remove old folders. When you run the repair
command, you have to find out which folder the app is currently using.

Use one fixed location instead. On startup, remove the old temporary database
and restore the saved backup there. If restoration fails, stop rather than use
old files. The saved backup and the original preview data must remain untouched.

**Consequence:** less cleanup code and a repair command with a predictable path.
This is the first change I recommend.

Code: internal/backup/backup.go, Prepare; docs/operations.md, repair instructions.

### 2. Stop making the app inspect how Linux has mounted its folders

Today, the app reads a Linux system file and works out whether its database folder
is covered by another mount. It can refuse to start based on that inspection.
That adds code tied to Linux and another reason startup can fail.

Keep the database unmounted through the Docker configuration, and check that in
the container tests we already have. Remove the inspection from the app.

**Consequence:** less startup code. A manually altered Docker setup would be the
operator's responsibility; the app would no longer detect that mistake itself.
The database must still be temporary. The user has approved this change.

Removed: validateRuntimeMount in internal/backup/platform_linux.go. The mount
check remains in scripts/verify-operations.py.

### 3. Remove the second way of preventing two backups running together

Today, the app already waits for one backup to finish before starting another.
It also takes a lock to prevent two backups running together.

Keep the existing order and remove that extra lock. Keep the separate lock that
allows the status page to read backup progress safely.

**Consequence:** a small reduction in code. Future callers must continue to run
backups one at a time. There is no need for a replacement queue or background service.

Code: internal/backup/backup.go, publishMu.

### 4. Copy an old database directly, without pretending it is an open database

The conversion tool currently builds a Store object containing just a filename.
The rest of that object is missing, so calling its normal database methods would
fail. It exists only to reuse the copy method.

Have both the running app and the conversion tool call a function that copies
from a filename.

**Consequence:** the code says what it actually does, and there is no half-built
object to misuse. The original database still must not be changed during conversion.

Code: internal/storage/snapshot.go, SnapshotSource.

### 5. Tell you why a backup failed

Today, several different failures become the same message telling you to check
space, permissions and the timeout. You have to investigate which one happened.

Keep the safe reason: for example, permission denied, disk full, damaged backup
or timeout. Keep secrets and private paths out of the message.

**Consequence:** you can act on an error without guessing. This needs clearer
messages, not a new error-handling system.

Code: internal/backup/backup.go, Publish; cmd/dump/main.go, main.

## What I would keep

- Copying SQLite while collection continues, with a time limit. Copying a live
  database like an ordinary file can produce an unusable backup.
- Finishing a new backup before replacing the previous one. A failed write must
  not destroy the only good copy. The Linux directory-sync code helps that
  replacement survive a power failure.
- Checking a backup before restoring it, and refusing to start empty when an
  established installation has lost its backup. Otherwise missing data can look
  like a successful fresh installation.
- Waiting for collection to stop before taking the final shutdown backup.
  Otherwise the last readings might miss that backup.
- Showing backup failures, briefly delaying aggregate requests after restoration,
  and providing a deliberate way to convert the old preview data. These deal with
  actual operating needs.
- Tests that check those outcomes. Many test lines do not, by themselves, mean
  the app is overcomplicated.

The single backup file and its contents can stay. I have not found a reason to
change the file format. The Linux-specific filename is also ordinary Go practice;
what matters is whether each function inside it earns its place.

## How to act on this

Approved work: change 1, plus fixed `snapshot.tmp` and `latest.tar.tmp` working
filenames and the standing engineering instructions. Check that restart still restores the saved data and
that a failed restore never starts the app using leftovers. Keep the preview untouched.

The other changes are proposals, not permission for me to do them all. For any
chosen change, remove the stated complication and keep the stated behavior. If I
find it cannot work, I must show the concrete problem before taking a different
approach. I must not quietly build a new system to replace the one being removed.
