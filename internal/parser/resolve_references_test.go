package parser

import "testing"

// TestResolveReferences_DoesNotMutateCache covers a crash on ordinary,
// spec-conformant files: a radio-button group whose kids point back to the
// parent field via /Parent. Resolution used to write resolved objects back into
// the shared object cache, turning the /Kids ↔ /Parent reference cycle into a
// Go pointer cycle; the next resolution of the same field then recursed until
// the process died with a stack overflow.
func TestResolveReferences_DoesNotMutateCache(t *testing.T) {
	r := openTestPDF(t,
		"<< /Type /Catalog /Pages 2 0 R /AcroForm << /Fields [4 0 R] >> >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /Annots [5 0 R 6 0 R] >>",
		"<< /FT /Btn /Ff 49152 /T (choice) /Kids [5 0 R 6 0 R] >>",
		"<< /Type /Annot /Subtype /Widget /Parent 4 0 R /P 3 0 R /AS /Off >>",
		"<< /Type /Annot /Subtype /Widget /Parent 4 0 R /P 3 0 R /AS /Off >>")

	field, err := r.GetObject(4)
	if err != nil {
		t.Fatal(err)
	}
	kid, err := r.GetObject(5)
	if err != nil {
		t.Fatal(err)
	}

	// Resolve both directions of the cycle, twice.
	for i := 0; i < 2; i++ {
		resolvedField, ok := r.ResolveReferences(field).(*Dictionary)
		if !ok {
			t.Fatalf("resolved field is not a dictionary")
		}
		kids, ok := resolvedField.Get("Kids").(*Array)
		if !ok || kids.Len() != 2 {
			t.Fatalf("resolved /Kids = %v, want 2 entries", resolvedField.Get("Kids"))
		}
		if _, ok := kids.Get(0).(*Dictionary); !ok {
			t.Fatalf("resolved /Kids[0] is %T, want *Dictionary", kids.Get(0))
		}
		if _, ok := r.ResolveReferences(kid).(*Dictionary).Get("Parent").(*Dictionary); !ok {
			t.Fatalf("resolved /Parent is not a dictionary")
		}
	}

	// The cached objects must still hold references, not resolved objects.
	cachedKids, _ := field.(*Dictionary).Get("Kids").(*Array)
	if _, ok := cachedKids.Get(0).(*IndirectReference); !ok {
		t.Errorf("cached field /Kids[0] is %T; resolution must not mutate cached objects", cachedKids.Get(0))
	}
	if _, ok := kid.(*Dictionary).Get("Parent").(*IndirectReference); !ok {
		t.Errorf("cached kid /Parent is %T; resolution must not mutate cached objects", kid.(*Dictionary).Get("Parent"))
	}
}

// TestResolveReferences_UnresolvableReferenceKept verifies a dangling
// reference is left in place for the caller to handle.
func TestResolveReferences_UnresolvableReferenceKept(t *testing.T) {
	r := openTestPDF(t, testCatalog,
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R >>")
	in := NewArrayFromSlice([]PdfObject{NewIndirectReference(99, 0)})
	out, ok := r.ResolveReferences(in).(*Array)
	if !ok || out.Len() != 1 {
		t.Fatalf("ResolveReferences returned %v", out)
	}
	if _, ok := out.Get(0).(*IndirectReference); !ok {
		t.Errorf("dangling reference resolved to %T, want it kept as *IndirectReference", out.Get(0))
	}
}
