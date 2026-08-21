/**
 * Canonical file entry contract shared by Manager, plugins, pane contracts,
 * and panel sources.
 *
 * Keep this structurally aligned with the Wails FileInfo wire model. Provider-
 * specific metadata belongs in explicit optional fields or derived interfaces;
 * do not add a catch-all index signature because it makes the contract stricter
 * for ordinary FileInfo values.
 */
export var FileType;
(function (FileType) {
    FileType["FILE"] = "FILE";
    FileType["DIRECTORY"] = "DIRECTORY";
    FileType["ARCHIVE"] = "ARCHIVE";
    FileType["SYMLINK"] = "SYMLINK";
})(FileType || (FileType = {}));
//# sourceMappingURL=file-info.js.map