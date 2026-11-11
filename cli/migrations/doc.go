// Package migrations holds the codemods that rewrite deprecated configuration into the form that replaces it (design
// section 8, VERSIONING.md "Deprecations ship with a codemod"). A codemod is a pure function: a decoded kind message in,
// a new one out, never a write to disk; Document wraps one for a YAML document so that numi migrate can apply it.
//
// The knowledge-to-tools codemod (deprecated in 1.3.0) rewrites the knowledge list of an AdapterSelector into tools, and the
// knowledge_tools of a Lockfile into pinned_tools. The KnowledgeConnection kind has no codemod of its own: its
// provider and pinned tools are the input of the other two (see PluginsByConnection), and the person deletes the file
// once nothing refers to it.
package migrations
