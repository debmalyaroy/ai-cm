
import sys

libs = ['fitz', 'PyPDF2', 'pdfplumber', 'pptx']
available = []

print("Checking libraries...")
for lib in libs:
    try:
        __import__(lib)
        available.append(lib)
        print(f"[OK] {lib}")
    except ImportError:
        print(f"[MISSING] {lib}")

if not available:
    print("No PDF libraries found.")
    sys.exit(1)
