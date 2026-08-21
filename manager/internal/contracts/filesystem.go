package contracts

import core "github.com/epikodelabs/reassembly/core/contracts"

type FileType = core.FileType

const (
	FileTypeFile      = core.FileTypeFile
	FileTypeArchive   = core.FileTypeArchive
	FileTypeDirectory = core.FileTypeDirectory
	FileTypeSymlink   = core.FileTypeSymlink
)

type FileInfo = core.FileInfo
type FileProbe = core.FileProbe
type ArchiveListing = core.ArchiveListing
type DirectoryListing = core.DirectoryListing
type DirectoryPage = core.DirectoryPage
type FileContent = core.FileContent
type OperationResult = core.OperationResult
type SearchResult = core.SearchResult
type DiskUsageEntry = core.DiskUsageEntry
type DiskUsageTypeStat = core.DiskUsageTypeStat
type DiskUsageResult = core.DiskUsageResult
type DiskUsageProgress = core.DiskUsageProgress
