const {visit} = require('unist-util-visit')

function rewriteUrl(url) {
  if (typeof url !== 'string' || !url.startsWith('/')) {
    return url
  }
  return `pathname://${url}`
}

module.exports = function remarkRootLinks() {
  return (tree) => {
    visit(tree, ['link', 'image'], (node) => {
      node.url = rewriteUrl(node.url)
    })
  }
}
