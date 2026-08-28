package notepad

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (s *Store) bootstrap() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logDebug("store bootstrap: notesDir=%s, uploadsDir=%s", s.notesDir, s.uploadsDir)

	if err := s.ensureNotesDirLocked(); err != nil {
		logError("ensure notes dir failed: %v", err)
		return err
	}
	if err := os.MkdirAll(s.uploadsDir, 0o755); err != nil {
		logError("ensure uploads dir failed: %v", err)
		return err
	}

	return s.migrateOldNotesLocked()
}

func (s *Store) ensureNotesDirLocked() error {
	return os.MkdirAll(s.notesDir, 0o755)
}

func (s *Store) writeNoteLocked(id string, content []byte) error {
	if err := s.ensureNotesDirLocked(); err != nil {
		return err
	}
	return os.WriteFile(s.notePath(id), content, 0o644)
}

func (s *Store) ensureLandingNote() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, err := s.readMetaLocked()
	if err != nil {
		return "", err
	}
	if len(meta.Notes) > 0 {
		sorted := sortNotesByUpdatedAt(meta.Notes)
		logDebug("landing note: %s (total: %d notes)", sorted[0].ID, len(meta.Notes))
		return sorted[0].ID, nil
	}

	now := time.Now().UnixMilli()
	note := Note{
		ID:        uuidString(),
		Title:     "新建笔记",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.writeNoteLocked(note.ID, []byte("")); err != nil {
		return "", err
	}

	meta.Notes = append(meta.Notes, note)
	if err := s.writeMetaLocked(meta); err != nil {
		return "", err
	}

	logDebug("created landing note: %s", note.ID)
	return note.ID, nil
}

func (s *Store) getNoteContent(id string) (Note, string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, err := s.readMetaLocked()
	if err != nil {
		return Note{}, "", false, err
	}

	index := findNoteIndex(meta.Notes, id)
	if index < 0 {
		logDebug("note not found in meta: %s", id)
		return Note{}, "", false, nil
	}

	content, err := s.readNoteContentLocked(id)
	if err != nil {
		logError("read note content failed: %s, err: %v", id, err)
		return Note{}, "", false, err
	}

	logDebug("get note: %s, title=%s, contentLen=%d", id, meta.Notes[index].Title, len(content))
	return meta.Notes[index], content, true, nil
}

func (s *Store) listNotes() ([]Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, err := s.readMetaLocked()
	if err != nil {
		return nil, err
	}

	sorted := sortNotesByUpdatedAt(meta.Notes)
	logDebug("list notes: total=%d", len(sorted))
	return sorted, nil
}

func (s *Store) createNote(title string) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, err := s.readMetaLocked()
	if err != nil {
		return Note{}, err
	}

	now := time.Now().UnixMilli()
	note := Note{
		ID:        uuidString(),
		Title:     defaultString(title, "新建笔记"),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.writeNoteLocked(note.ID, []byte("")); err != nil {
		logError("write note file failed: %s, err: %v", note.ID, err)
		return Note{}, err
	}

	meta.Notes = append(meta.Notes, note)
	if err := s.writeMetaLocked(meta); err != nil {
		logError("write meta after create failed: %v", err)
		return Note{}, err
	}

	logDebug("create note: %s, title=%s, total=%d", note.ID, note.Title, len(meta.Notes))
	return note, nil
}

func (s *Store) deleteNote(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, err := s.readMetaLocked()
	if err != nil {
		return false, err
	}

	index := findNoteIndex(meta.Notes, id)
	if index < 0 {
		logDebug("delete note not found: %s", id)
		return false, nil
	}

	note := meta.Notes[index]
	for _, filename := range note.Attachments {
		if err := s.deleteAttachmentLocked(filename); err != nil {
			logError("delete attachment failed: %s, err: %v", filename, err)
			return false, err
		}
	}

	meta.Notes = append(meta.Notes[:index], meta.Notes[index+1:]...)
	if err := s.writeMetaLocked(meta); err != nil {
		logError("write meta after delete failed: %v", err)
		return false, err
	}

	if err := os.Remove(s.notePath(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		logError("remove note file failed: %s, err: %v", id, err)
		return false, err
	}

	logDebug("delete note: %s, wasTitle=%s, remaining=%d", id, note.Title, len(meta.Notes))
	return true, nil
}

func (s *Store) renameNote(id, title string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, err := s.readMetaLocked()
	if err != nil {
		return "", false, err
	}

	index := findNoteIndex(meta.Notes, id)
	if index < 0 {
		return "", false, nil
	}

	meta.Notes[index].Title = defaultString(title, "无标题")
	markNoteAsEdited(&meta.Notes[index])
	if err := s.writeMetaLocked(meta); err != nil {
		return "", false, err
	}

	return meta.Notes[index].Title, true, nil
}

func (s *Store) saveNoteContent(id, content string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, err := s.readMetaLocked()
	if err != nil {
		return false, err
	}

	index := findNoteIndex(meta.Notes, id)
	if index < 0 {
		logDebug("save note not found: %s", id)
		return false, nil
	}

	if err := s.writeNoteLocked(id, []byte(content)); err != nil {
		logError("write note content failed: %s, err: %v", id, err)
		return false, err
	}

	currentAttachments := extractAttachments(content)
	oldAttachments := meta.Notes[index].Attachments
	for _, filename := range oldAttachments {
		if !containsString(currentAttachments, filename) {
			if err := s.deleteAttachmentLocked(filename); err != nil {
				logError("delete stale attachment failed: %s, err: %v", filename, err)
				return false, err
			}
		}
	}

	meta.Notes[index].Attachments = currentAttachments
	markNoteAsEdited(&meta.Notes[index])
	if err := s.writeMetaLocked(meta); err != nil {
		logError("write meta after save failed: %v", err)
		return false, err
	}

	logDebug("save note: %s, contentLen=%d, attachments=%d", id, len(content), len(currentAttachments))
	return true, nil
}

func (s *Store) addAttachment(noteID, filename string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, err := s.readMetaLocked()
	if err != nil {
		return err
	}

	index := findNoteIndex(meta.Notes, noteID)
	if index < 0 {
		logDebug("add attachment: note not found %s", noteID)
		return nil
	}

	meta.Notes[index].Attachments = append(meta.Notes[index].Attachments, filename)
	logDebug("add attachment: note=%s, file=%s", noteID, filename)
	return s.writeMetaLocked(meta)
}

func (s *Store) migrateOldNotesLocked() error {
	meta, err := s.readMetaLocked()
	if err != nil {
		return err
	}
	if meta.Migrated {
		logDebug("old notes migration: already migrated, skip")
		return nil
	}

	logInfo("starting old notes migration...")
	baseTime := time.Now().UnixMilli()
	migratedCount := 0
	for i := 1; i <= 8; i++ {
		oldPath := filepath.Join(s.notesDir, fmt.Sprintf("%d.txt", i))
		content, err := os.ReadFile(oldPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}

		if strings.TrimSpace(string(content)) != "" {
			id := uuidString()
			if err := s.writeNoteLocked(id, content); err != nil {
				return err
			}

			timestamp := baseTime - int64(i*1000)
			meta.Notes = append(meta.Notes, Note{
				ID:        id,
				Title:     fmt.Sprintf("笔记 %d", i),
				CreatedAt: timestamp,
				UpdatedAt: timestamp,
			})
			migratedCount++
		}

		if err := os.Remove(oldPath); err != nil {
			return err
		}
	}

	meta.Migrated = true
	if err := s.writeMetaLocked(meta); err != nil {
		return err
	}

	logInfo("old notes migration done, migrated %d notes", migratedCount)
	return nil
}

func (s *Store) readMetaLocked() (Meta, error) {
	data, err := os.ReadFile(s.metaFile)
	if errors.Is(err, os.ErrNotExist) {
		return Meta{Notes: []Note{}}, nil
	}
	if err != nil {
		return Meta{}, err
	}

	var meta Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return Meta{}, err
	}
	if meta.Notes == nil {
		meta.Notes = []Note{}
	}

	return meta, nil
}

func (s *Store) writeMetaLocked(meta Meta) error {
	if meta.Notes == nil {
		meta.Notes = []Note{}
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := s.ensureNotesDirLocked(); err != nil {
		return err
	}
	return os.WriteFile(s.metaFile, data, 0o644)
}

func (s *Store) readNoteContentLocked(id string) (string, error) {
	data, err := os.ReadFile(s.notePath(id))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Store) deleteAttachmentLocked(filename string) error {
	name := filepath.Base(filename)
	if name == "." || name == "" {
		return nil
	}

	err := os.Remove(filepath.Join(s.uploadsDir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Store) notePath(id string) string {
	return filepath.Join(s.notesDir, id+".txt")
}

func effectiveUpdatedAt(note Note) int64 {
	if note.UpdatedAt != 0 {
		return note.UpdatedAt
	}
	return note.CreatedAt
}

func sortNotesByUpdatedAt(notes []Note) []Note {
	sorted := append([]Note(nil), notes...)
	sort.Slice(sorted, func(i, j int) bool {
		leftUpdated := effectiveUpdatedAt(sorted[i])
		rightUpdated := effectiveUpdatedAt(sorted[j])
		if leftUpdated != rightUpdated {
			return leftUpdated > rightUpdated
		}
		return sorted[i].CreatedAt > sorted[j].CreatedAt
	})
	return sorted
}

func markNoteAsEdited(note *Note) {
	note.UpdatedAt = time.Now().UnixMilli()
}

func extractAttachments(content string) []string {
	matches := attachmentPattern.FindAllStringSubmatch(content, -1)
	attachments := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			attachments = append(attachments, match[1])
		}
	}
	return attachments
}

func findNoteIndex(notes []Note, id string) int {
	for index, note := range notes {
		if note.ID == id {
			return index
		}
	}
	return -1
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}