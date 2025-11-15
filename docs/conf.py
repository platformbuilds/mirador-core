project = 'MIRADOR-CORE'
copyright_str = '2025, PlatformBuilds Global Private Limited'
author = 'PlatformBuilds Team'
release = '9.0.0'

# Set copyright using the string variable
copyright = copyright_str

# -- General configuration ---------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#general-configuration

extensions = [
    'sphinx.ext.autodoc',
    'sphinx.ext.viewcode',
    'sphinx.ext.napoleon',
    'sphinx.ext.intersphinx',
    'sphinx.ext.todo',
    'sphinx.ext.githubpages',
    'myst_parser',  # For Markdown support
]

templates_path = ['_templates']
exclude_patterns = ['_build', 'Thumbs.db', '.DS_Store']

# -- Options for HTML output -------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#options-for-html-output

html_theme = 'sphinx_rtd_theme'
html_static_path = ['_static']
html_logo = '_static/logo.svg'

# -- Options for intersphinx extension ---------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#options-for-intersphinx

intersphinx_mapping = {
    'python': ('https://docs.python.org/3', None),
    # Note: Go documentation doesn't provide intersphinx support
    # Removed golang intersphinx mapping to avoid 404 errors
}

# -- Options for MyST (Markdown) parser -------------------------------------
# https://myst-parser.readthedocs.io/en/latest/configuration.html

myst_enable_extensions = [
    'colon_fence',
    'deflist',
    'dollarmath',
    'fieldlist',
    'html_admonition',
    'html_image',
    'replacements',
    'smartquotes',
    'strikethrough',
    'substitution',
    'tasklist',
]

myst_heading_anchors = 3

# -- Custom configuration ---------------------------------------------------

# Add any custom configuration here