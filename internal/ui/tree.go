package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ryo246912/lazychezmoi/tools/lazychezmoi/internal/model"
)

type listRow struct {
	key        string
	targetPath string
	name       string
	depth      int
	directory  bool
	expanded   bool
	entry      model.Entry
	hasEntry   bool
}

type treeNode struct {
	key        string
	targetPath string
	name       string
	directory  bool
	entry      *model.Entry
	children   map[string]*treeNode
}

func newTreeNode(key, targetPath, name string, directory bool) *treeNode {
	return &treeNode{
		key:        key,
		targetPath: targetPath,
		name:       name,
		directory:  directory,
		children:   make(map[string]*treeNode),
	}
}

func (m Model) selectedRow() *listRow {
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	return &m.rows[m.cursor]
}

func (m Model) selectedRowKey() string {
	row := m.selectedRow()
	if row == nil {
		return ""
	}
	return row.key
}

func (m Model) rowIndex(key string) int {
	for i := range m.rows {
		if m.rows[i].key == key {
			return i
		}
	}
	return -1
}

func (m Model) firstEntryRowIndex() int {
	for i := range m.rows {
		if m.rows[i].hasEntry {
			return i
		}
	}
	if len(m.rows) == 0 {
		return -1
	}
	return 0
}

func (m *Model) rebuildRows(anchorKey string) {
	m.rows = m.buildRows()
	switch {
	case anchorKey != "":
		if idx := m.rowIndex(anchorKey); idx >= 0 {
			m.cursor = idx
			break
		}
		fallthrough
	default:
		if idx := m.firstEntryRowIndex(); idx >= 0 {
			m.cursor = idx
		} else {
			m.cursor = 0
		}
	}
	m.clampCursor()
}

func (m Model) buildRows() []listRow {
	entries := m.allEntries()
	if len(entries) == 0 {
		return nil
	}

	rootPath := commonParentPath(entries)
	root := newTreeNode("root", rootPath, "", true)
	for _, entry := range entries {
		m.insertTreeEntry(root, rootPath, entry)
	}

	return m.flattenRows(root, 0)
}

func (m Model) flattenRows(node *treeNode, depth int) []listRow {
	children := sortedChildren(node.children)
	rows := make([]listRow, 0, len(children))
	for _, child := range children {
		row := listRow{
			key:        child.key,
			targetPath: child.targetPath,
			name:       child.name,
			depth:      depth,
			directory:  child.directory || len(child.children) > 0,
			expanded:   m.isDirectoryExpanded(child),
		}
		if child.entry != nil {
			row.entry = *child.entry
			row.hasEntry = true
		}

		childRows := []listRow{row}
		if row.directory && (row.expanded || m.filterQuery != "") {
			childRows = append(childRows, m.flattenRows(child, depth+1)...)
		}

		if m.filterQuery != "" && !m.rowsMatchFilter(row, childRows[1:]) {
			continue
		}
		rows = append(rows, childRows...)
	}
	return rows
}

func (m Model) rowsMatchFilter(row listRow, descendants []listRow) bool {
	if m.rowMatchesFilter(row) {
		return true
	}
	for _, child := range descendants {
		if m.rowMatchesFilter(child) {
			return true
		}
	}
	return false
}

func (m Model) rowMatchesFilter(row listRow) bool {
	query := strings.TrimSpace(strings.ToLower(m.filterQuery))
	if query == "" {
		return true
	}

	candidates := []string{strings.ToLower(row.name), strings.ToLower(row.targetPath)}
	if row.hasEntry && row.entry.SourcePath != "" {
		candidates = append(candidates, strings.ToLower(row.entry.SourcePath))
	}
	for _, candidate := range candidates {
		if strings.Contains(candidate, query) {
			return true
		}
	}
	return false
}

func (m Model) allEntries() []model.Entry {
	entries := make([]model.Entry, 0, len(m.entries))
	entries = append(entries, m.entries...)
	for _, children := range m.dirChildren {
		entries = append(entries, children...)
	}
	return entries
}

func (m Model) allTargetPaths() []string {
	entries := m.allEntries()
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.TargetPath)
	}
	return paths
}

func (m *Model) insertTreeEntry(root *treeNode, rootPath string, entry model.Entry) {
	segments := relativeSegments(rootPath, entry.TargetPath)
	if len(segments) == 0 {
		segments = []string{filepath.Base(entry.TargetPath)}
	}

	current := root
	for i, segment := range segments {
		fullPath := joinRootPath(rootPath, segments[:i+1])
		child, ok := current.children[segment]
		if !ok {
			child = newTreeNode(fullPath, fullPath, segment, i < len(segments)-1)
			current.children[segment] = child
		}
		if i == len(segments)-1 {
			child.directory = entry.TargetType == model.TargetDirectory || len(child.children) > 0
			entryCopy := entry
			child.entry = &entryCopy
		}
		current = child
	}
}

func (m Model) isDirectoryExpanded(node *treeNode) bool {
	if !node.directory && len(node.children) == 0 {
		return false
	}
	if expanded, ok := m.expandedDirs[node.targetPath]; ok {
		return expanded
	}
	if node.entry == nil {
		return true
	}
	return node.entry.Kind == model.EntryManaged
}

func (m *Model) toggleDirectory(row listRow) error {
	if !row.directory {
		return nil
	}

	nextExpanded := !row.expanded
	if nextExpanded && row.hasEntry && row.entry.Kind == model.EntryUnmanaged && row.entry.TargetType == model.TargetDirectory {
		if _, ok := m.dirChildren[row.targetPath]; !ok {
			children, err := loadDirectoryChildren(row.targetPath, m.allEntries())
			if err != nil {
				return err
			}
			m.dirChildren[row.targetPath] = children
		}
	}
	m.expandedDirs[row.targetPath] = nextExpanded
	return nil
}

func loadDirectoryChildren(path string, known []model.Entry) ([]model.Entry, error) {
	knownByPath := make(map[string]model.Entry, len(known))
	for _, entry := range known {
		knownByPath[entry.TargetPath] = entry
	}

	items, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", path, err)
	}

	children := make([]model.Entry, 0, len(items))
	for _, item := range items {
		childPath := filepath.Join(path, item.Name())
		if entry, ok := knownByPath[childPath]; ok {
			children = append(children, entry)
			continue
		}
		children = append(children, model.Entry{
			Kind:       model.EntryUnmanaged,
			TargetPath: childPath,
			TargetType: model.DetectTargetKind(childPath),
		})
	}

	sort.Slice(children, func(i, j int) bool {
		leftDir := children[i].TargetType == model.TargetDirectory
		rightDir := children[j].TargetType == model.TargetDirectory
		if leftDir != rightDir {
			return leftDir
		}
		return children[i].TargetPath < children[j].TargetPath
	})

	return children, nil
}

func relativeSegments(rootPath, targetPath string) []string {
	if rootPath == "" {
		return splitPath(targetPath)
	}
	rel, err := filepath.Rel(rootPath, targetPath)
	if err != nil {
		return splitPath(targetPath)
	}
	return splitPath(rel)
}

func splitPath(path string) []string {
	path = filepath.Clean(path)
	if path == "." || path == string(filepath.Separator) {
		return nil
	}
	parts := strings.Split(path, string(filepath.Separator))
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		segments = append(segments, part)
	}
	return segments
}

func joinRootPath(rootPath string, segments []string) string {
	if len(segments) == 0 {
		return rootPath
	}
	if rootPath == "" {
		return filepath.Join(segments...)
	}
	parts := append([]string{rootPath}, segments...)
	return filepath.Join(parts...)
}

func commonParentPath(entries []model.Entry) string {
	if len(entries) == 0 {
		return ""
	}

	common := filepath.Dir(entries[0].TargetPath)
	for _, entry := range entries[1:] {
		common = commonPath(common, filepath.Dir(entry.TargetPath))
	}
	return common
}

func commonPath(left, right string) string {
	leftParts := splitPath(left)
	rightParts := splitPath(right)
	limit := min(len(leftParts), len(rightParts))

	var common []string
	for i := 0; i < limit; i++ {
		if leftParts[i] != rightParts[i] {
			break
		}
		common = append(common, leftParts[i])
	}

	if len(common) == 0 {
		return string(filepath.Separator)
	}
	return string(filepath.Separator) + filepath.Join(common...)
}

func sortedChildren(children map[string]*treeNode) []*treeNode {
	list := make([]*treeNode, 0, len(children))
	for _, child := range children {
		list = append(list, child)
	}
	sort.Slice(list, func(i, j int) bool {
		leftDir := list[i].directory || len(list[i].children) > 0
		rightDir := list[j].directory || len(list[j].children) > 0
		if leftDir != rightDir {
			return leftDir
		}
		return strings.ToLower(list[i].name) < strings.ToLower(list[j].name)
	})
	return list
}
