import api from '@/api/index.js'

const resources = {
  state: {
    resources: [],
    error: null
  },

  mutations: {
    SET_RESOURCES: (state, resources) => {
      state.resources = []
      for (var i = 0; i < resources.length; i++) {
        const bowOpts = {}

        const approvals = resources[i].annotations['bow.sh/approvals']
        if (approvals) {
          resources[i]._required_approvals = approvals
        }

        const triggerAnnotation = resources[i].annotations['bow.sh/trigger']
        if (triggerAnnotation) {
          resources[i]._trigger_poll = triggerAnnotation === 'poll'
        } else {
          // additional check for labels
          const triggerLabel = resources[i].labels['bow.sh/trigger']
          if (triggerLabel) {
            resources[i]._trigger_poll = triggerLabel === 'poll'
          } else {
            resources[i]._trigger_poll = false
          }
        }

        const labels = resources[i].labels
        for (var label in labels) {
          if (labels.hasOwnProperty(label)) {
            if (label.startsWith('bow.sh/')) {
              bowOpts[label] = labels[label]
            }
          }
        }

        const annotations = resources[i].annotations
        for (var annotation in annotations) {
          if (annotations.hasOwnProperty(annotation)) {
            if (annotation.startsWith('bow.sh/')) {
              bowOpts[annotation] = annotations[annotation]
            }
          }
        }

        resources[i]._bow_opts = bowOpts
      }
      state.resources = resources
    },
    SET_ERROR: (state, error) => {
      state.error = error
    },
    SET_RESOURCE_LOADING: (state, identifier) => {
      var arrayLength = state.resources.length
      for (var i = 0; i < arrayLength; i++) {
        if (state.resources[i].identifier === identifier) {
          state.resources[i]._loading = true
        }
      }
    }
  },

  actions: {
    GetResources ({ commit }) {
      commit('SET_ERROR', null)
      return api.get('resources')
        .then((response) => {
          commit('SET_RESOURCES', response)
        })
        .catch((error) => commit('SET_ERROR', error))
    },
    SetResourcePolicy ({ commit }, payload) {
      commit('SET_ERROR', null)
      commit('SET_RESOURCE_LOADING', payload.identifier)
      return api.put(`policies`, payload)
        .then((response) => commit('SET_ERROR', null))
        .catch((error) => commit('SET_ERROR', error))
    }
  }
}

export default resources
