
import fitz  # PyMuPDF
import os
import re

# Configuration
PDF_PATH = r"c:\Users\debma\OneDrive\Desktop\development\ai-cm\AI-CM an Agentic Autopilot for Category Manager.pdf"
OUTPUT_HTML = r"c:\Users\debma\OneDrive\Desktop\development\ai-cm\docs\presentation.html"
ASSETS_DIR = r"c:\Users\debma\OneDrive\Desktop\development\ai-cm\docs\assets\slides"

# Reveal.js Template
HTML_HEADER = """<!doctype html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>AI-CM Presentation</title>
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/reveal.js/5.0.4/reset.min.css">
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/reveal.js/5.0.4/reveal.min.css">
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/reveal.js/5.0.4/theme/black.min.css">
    <style>
        .reveal h1, .reveal h2, .reveal h3 { color: #d4b067; }
        .slide-content { text-align: left; font-size: 0.7em; }
        .slide-image { max-height: 400px; max-width: 100%; margin: 20px auto; display: block; border-radius: 8px; }
        .reveal ul { display: block; }
    </style>
</head>
<body>
    <div class="reveal">
        <div class="slides">
"""

HTML_FOOTER = """
        </div>
    </div>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/reveal.js/5.0.4/reveal.min.js"></script>
    <script>
        Reveal.initialize({
            hash: true,
            slideNumber: true,
        });
    </script>
</body>
</html>
"""

def extract_and_generate():
    if not os.path.exists(ASSETS_DIR):
        os.makedirs(ASSETS_DIR)

    try:
        doc = fitz.open(PDF_PATH)
    except Exception as e:
        print(f"Error opening PDF: {e}")
        return

    slides_html = ""

    for page_num, page in enumerate(doc):
        print(f"Processing Page {page_num + 1}...")
        
        # 1. Extract Text
        text = page.get_text("text")
        lines = [line.strip() for line in text.split('\n') if line.strip()]
        
        # Simple heuristic: First non-empty line is title
        title = lines[0] if lines else f"Slide {page_num + 1}"
        body_lines = lines[1:] if len(lines) > 1 else []
        
        # 2. Extract Images
        images_html = ""
        image_list = page.get_images(full=True)
        for img_index, img in enumerate(image_list):
            xref = img[0]
            base_image = doc.extract_image(xref)
            image_bytes = base_image["image"]
            ext = base_image["ext"]
            
            image_filename = f"slide_{page_num+1}_img_{img_index+1}.{ext}"
            image_path = os.path.join(ASSETS_DIR, image_filename)
            
            with open(image_path, "wb") as f:
                f.write(image_bytes)
            
            # Relative path for HTML
            rel_path = f"assets/slides/{image_filename}"
            images_html += f'<img src="{rel_path}" class="slide-image" alt="Slide Image">'

        # 3. Construct Slide HTML
        # Convert body lines to bullets if they look like items, otherwise paragraphs
        body_html = "<ul>"
        for line in body_lines:
            if len(line) > 80: # Long formatting
                body_html += f"</ul><p>{line}</p><ul>"
            else:
                body_html += f"<li>{line}</li>"
        body_html += "</ul>"
        
        # Clean up empty tags
        body_html = body_html.replace("<ul></ul>", "")

        slide_block = f"""
            <section>
                <h2>{title}</h2>
                <div class="slide-content">
                    {body_html}
                </div>
                {images_html}
            </section>
        """
        slides_html += slide_block

    # Write Output
    full_html = HTML_HEADER + slides_html + HTML_FOOTER
    
    with open(OUTPUT_HTML, "w", encoding="utf-8") as f:
        f.write(full_html)
    
    print(f"Success! Generated presentation at: {OUTPUT_HTML}")

if __name__ == "__main__":
    extract_and_generate()
