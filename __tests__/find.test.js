const { find } = require('../find.js')
jest.mock('../getDepsSet.js')
const getDepsSet = require('../getDepsSet')

test('It throws error for nonexisting dependency', () => {
  getDepsSet.mockImplementation(() => [])
  expect(() =>
    find({
      entryPoints: ['index.js'],
      file: 'file.js'
    })
  ).toThrow()
})

test('It should resolve path for simple tree', () => {
  getDepsSet.mockImplementation(() => [
    {
      source: 'index.js',
      dependencies: [
        {
          resolved: 'file.js'
        }
      ]
    },
    {
      source: 'file.js',
      dependencies: []
    }
  ])
  const result = find({
    entryPoints: ['index.js'],
    file: 'file.js'
  })
  expect(result).toMatchObject([[['index.js', 'file.js']]])
})

test('It should find two paths for the same file', () => {
  getDepsSet.mockImplementation(() => [
    {
      source: 'index.js',
      dependencies: [
        {
          resolved: 'file.js'
        }
      ]
    },
    {
      source: 'index2.js',
      dependencies: [
        {
          resolved: 'file.js'
        }
      ]
    },
    {
      source: 'file.js',
      dependencies: []
    }
  ])
  const result = find({
    entryPoints: ['index.js', 'index2.js'],
    file: 'file.js'
  })
  expect(result).toMatchObject([
    [['index.js', 'file.js']],
    [['index2.js', 'file.js']]
  ])
})

test('It should find path for a file via another file', () => {
  getDepsSet.mockImplementation(() => [
    {
      source: 'index.js',
      dependencies: [
        {
          resolved: 'module.js'
        }
      ]
    },
    {
      source: 'module.js',
      dependencies: [
        {
          resolved: 'file.js'
        }
      ]
    },
    {
      source: 'file.js',
      dependencies: []
    }
  ])
  const result = find({
    entryPoints: ['index.js'],
    file: 'file.js'
  })
  expect(result).toMatchObject([[['index.js', 'module.js', 'file.js']]])
})

test('It should find paths for a file for entry points in different directories', () => {
  getDepsSet.mockImplementation(() => [
    {
      source: 'index.js',
      dependencies: [
        {
          resolved: 'file.js'
        }
      ]
    },
    {
      source: 'dir/index.js',
      dependencies: [
        {
          resolved: 'file.js'
        }
      ]
    },
    {
      source: 'file.js',
      dependencies: []
    }
  ])
  const result = find({
    entryPoints: ['index.js', 'dir/index.js'],
    file: 'file.js'
  })
  expect(result).toMatchObject([
    [['index.js', 'file.js']],
    [['dir/index.js', 'file.js']]
  ])
})
