/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */
import { useCallback, useEffect, useState, useMemo } from 'react'
import { ToastNotification } from '@/components/Toast'
import { DEVLAKE_ENDPOINT } from '@/utils/config'
import request from '@/utils/request'
import { NullBlueprint, BlueprintMode } from '@/data/NullBlueprint'
import { DEFAULT_DATA_ENTITIES } from '@/data/BlueprintWorkflow'
import { integrationsData } from '@/data/integrations'
import TransformationSettings from '@/models/TransformationSettings'
import JiraBoard from '@/models/JiraBoard'
import GitHubProject from '@/models/GithubProject'
import GitlabProject from '@/models/GitlabProject'
import { Providers, ProviderLabels, ProviderIcons } from '@/data/Providers'
import { DataScopeModes } from '@/data/DataScopes'

function useDataScopesManager({
  mode = DataScopeModes.CREATE,
  provider,
  blueprint,
  /* connection, */ settings = {},
  setSettings = () => {}
}) {
  const [connections, setConnections] = useState([])
  const [newConnections, setNewConnections] = useState([])

  const [scopeConnection, setScopeConnection] = useState()
  const [configuredConnection, setConfiguredConnection] = useState()
  const connection = useMemo(
    () =>
      mode === DataScopeModes.EDIT ? scopeConnection : configuredConnection,
    [scopeConnection, configuredConnection, mode]
  )
  // const connection = useMemo(() => scopeConnection, [scopeConnection])

  // const [blueprint, setBlueprint] = useState(NullBlueprint)
  const [boards, setBoards] = useState({})
  const [projects, setProjects] = useState({})
  const [entities, setEntities] = useState({})
  const [transformations, setTransformations] = useState({})
  const [enabledProviders, setEnabledProviders] = useState([])

  const [configuredProject, setConfiguredProject] = useState(null)
  const [configuredBoard, setConfiguredBoard] = useState(null)
  const configurationKey = useMemo(() => {
    let key = `C#${connection?.id}`
    switch (connection?.providerId) {
      case Providers.GITHUB:
      case Providers.GITLAB:
        key = configuredProject?.id
        break
      case Providers.JIRA:
        key = configuredBoard?.id
        break
      case Providers.JENKINS:
      case 'default':
        key = `C#${connection?.id}`
        break
    }
    console.log('>>> DSM: Active Configuration Key ===', key)
    return key
  }, [
    connection?.providerId,
    connection?.id,
    configuredProject?.id,
    configuredBoard?.id
  ])

  const activeProject = useMemo(() => configuredProject, [configuredProject])
  const activeBoard = useMemo(() => configuredBoard, [configuredBoard])

  const selectedProjects = useMemo(
    () => projects[connection?.id]?.map((p) => p && p?.id),
    [projects, connection?.id]
  )
  const selectedBoards = useMemo(
    () => boards[connection?.id]?.map((b) => b && b?.id),
    [boards, connection?.id]
  )

  const storedProjectTransformation = useMemo(
    () =>
      connection?.transformations &&
      connection?.transformations[
        connection?.projects?.findIndex((p) => p === configuredProject?.id)
      ],
    [connection, configuredProject?.id]
  )
  const storedBoardTransformation = useMemo(
    () =>
      connection?.transformations &&
      connection?.transformations[
        connection?.boardIds?.findIndex((b) => b === configuredBoard?.id)
      ],
    [connection, configuredBoard?.id]
  )

  const activeProjectTransformation = useMemo(
    () => transformations[activeProject?.id],
    [transformations, activeProject?.id]
  )
  const activeBoardTransformation = useMemo(
    () => transformations[activeBoard?.id],
    [transformations, activeBoard?.id]
  )
  const activeTransformation = useMemo(
    () => transformations[configurationKey],
    [configurationKey, transformations]
  )
  // eslint-disable-next-line max-len
  // const activeTransformation = useMemo(() => transformations[connection?.providerId === Providers.JIRA ? configuredBoard?.id : configuredProject?.id], [transformations, configuredProject?.id, configuredBoard?.id, connection?.providerId])

  const getDefaultTransformations = useCallback((providerId) => {
    let transforms = {}
    switch (providerId) {
      case Providers.GITHUB:
        transforms = {
          prType: '',
          prComponent: '',
          prBodyClosePattern: '',
          issueSeverity: '',
          issueComponent: '',
          issuePriority: '',
          issueTypeRequirement: '',
          issueTypeBug: '',
          issueTypeIncident: '',
          refdiff: null,
          productionPattern: '',
          deploymentPattern: ''
          // stagingPattern: '',
          // testingPattern: ''
        }
        break
      case Providers.JIRA:
        transforms = {
          epicKeyField: '',
          typeMappings: {},
          storyPointField: '',
          remotelinkCommitShaPattern: '',
          bugTags: [],
          incidentTags: [],
          requirementTags: [],
          // @todo: verify if jira utilizes deploy tag(s)?
          productionPattern: '',
          deploymentPattern: ''
          // stagingPattern: '',
          // testingPattern: ''
        }
        break
      case Providers.JENKINS:
        transforms = {
          productionPattern: '',
          deploymentPattern: ''
          // stagingPattern: '',
          // testingPattern: ''
        }
        break
      case Providers.GITLAB:
        transforms = {
          productionPattern: '',
          deploymentPattern: ''
          // stagingPattern: '',
          // testingPattern: ''
        }
        break
      case Providers.TAPD:
        // @todo: complete tapd transforms #2673
        transforms = {
          issueTypeRequirement: '',
          issueTypeBug: '',
          issueTypeIncident: '',
          productionPattern: '',
          deploymentPattern: ''
          // stagingPattern: '',
          // testingPattern: ''
        }
        break
    }
    console.log(
      '>>>>> DATA SCOPES MANAGER: Getting Default Transformation Values for PROVIDER Type ',
      providerId,
      transforms
    )
    return transforms
  }, [])

  const initializeTransformations = useCallback(
    (pV, cV, iDx) => ({
      ...pV,
      [cV]: new TransformationSettings(
        getDefaultTransformations(connection?.providerId, iDx)
      )
    }),
    [connection?.providerId, getDefaultTransformations]
  )

  // @todo: generate scopes dynamically from $integrationsData (in future Integrations Hook [plugin registry])
  const createProviderScopes = useCallback(
    (
      providerId,
      connection,
      connectionIdx,
      entities = {},
      boards = {},
      projects = {},
      transformations = {},
      defaultScope = { transformation: {}, options: {}, entities: [] }
    ) => {
      console.log(
        '>>> DATA SCOPES MANAGER: CREATING PROVIDER SCOPE FOR CONNECTION...',
        connectionIdx,
        connection
      )
      let newScope = {
        ...defaultScope,
        entities: entities[connection.id]?.map((entity) => entity.value) || []
      }
      switch (providerId) {
        case Providers.JIRA:
          newScope = boards[connection.id]?.map((b) => ({
            ...newScope,
            options: {
              boardId: Number(b?.value),
              title: b.title
              // @todo: verify initial value of since date for jira provider
              // since: new Date(),
            },
            transformation: { ...transformations[b?.id] }
          }))
          break
        case Providers.GITLAB:
          newScope = projects[connection.id]?.map((p) => ({
            ...newScope,
            options: {
              projectId: Number(p.value),
              title: p.title
            },
            transformation: { ...transformations[p?.id] }
          }))
          break
        case Providers.JENKINS:
          newScope = {
            ...newScope,
            // options: {
            // },
            // NOTE: Jenkins has no concept of projects/boards. Transformations Key'ed by Conn *INDEX* ID!
            transformation: { ...transformations[`C#${connection?.id}`] }
          }
          break
        case Providers.GITHUB:
          newScope = projects[connection.id]?.map((p) => ({
            ...newScope,
            options: {
              owner: p.value.split('/')[0],
              repo: p.value.split('/')[1]
            },
            transformation: { ...transformations[p?.id] }
          }))
          break
        case Providers.TAPD:
          newScope = {
            ...newScope
            // options: {
            // },
            // transformation: {},
          }
          break
      }
      return Array.isArray(newScope) ? newScope.flat() : [newScope]
    },
    []
  )

  const createProviderConnections = useCallback(
    (blueprintConnections) => {
      console.log(
        '>>>>> DATA SCOPES MANAGER: Creating Provider Connection Scopes...',
        blueprintConnections
      )
      return blueprintConnections.map((c, cIdx) => ({
        connectionId: c.value || c.connectionId,
        plugin: c.plugin || c.provider,
        scope: createProviderScopes(
          typeof c.provider === 'object' ? c.provider?.id : c.provider,
          c,
          cIdx,
          entities,
          boards,
          projects,
          transformations
        )
      }))
    },
    [boards, projects, entities, transformations, createProviderScopes]
  )

  const modifyConnectionSettings = useCallback(() => {
    const newConnections = createProviderConnections(
      connections.filter((c) => c.providerId === connection?.providerId)
    )
    const existingConnections = blueprint?.settings?.connections?.filter(
      (storedConnection) => storedConnection.plugin !== connection?.plugin
    )
    console.log(
      '>>>>> DATA SCOPES MANAGER: Modifying Connection Scopes...',
      newConnections
    )
    console.log(
      '>>>>> DATA SCOPES MANAGER: Filtered Existing connection Scopes...',
      existingConnections
    )
    setSettings((currentSettings) => ({
      ...currentSettings,
      connections: [...newConnections, ...existingConnections]
    }))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    connection,
    connections,
    boards,
    projects,
    entities,
    transformations,
    blueprint?.settings?.connections,
    setSettings,
    createProviderConnections
  ])

  const setTransformationSettings = useCallback(
    (settings, configuredEntity) => {
      console.log(
        '>>>>> DATA SCOPES MANAGER: SETTING TRANSFORMATION SETTINGS PROJECT/BOARD...',
        configuredEntity,
        settings
      )
      setTransformations((existingTransformations) =>
        configuredEntity
          ? {
              ...existingTransformations,
              [configuredEntity]: new TransformationSettings({
                ...existingTransformations[configuredEntity],
                ...settings
              })
            }
          : existingTransformations
      )
    },
    [setTransformations]
  )

  const getGithubProjects = useCallback(
    (c) =>
      [Providers.GITHUB].includes(c.plugin)
        ? c.scope.map(
            (s) =>
              new GitHubProject({
                id: `${s.options?.owner}/${s.options?.repo}`,
                key: `${s.options?.owner}/${s.options?.repo}`,
                owner: s.options?.owner,
                repo: s.options?.repo,
                value: `${s.options?.owner}/${s.options?.repo}`,
                title: `${s.options?.owner}/${s.options?.repo}`
              })
          )
        : [],
    []
  )

  const getGitlabProjects = useCallback(
    (c) =>
      [Providers.GITLAB].includes(c.plugin)
        ? c.scope.map(
            (s) =>
              new GitlabProject({
                id: s.options?.projectId,
                key: s.options?.projectId,
                value: s.options?.projectId,
                title: s.options?.title || `Project ${s.options?.projectId}`
              })
          )
        : [],
    []
  )

  const getAdvancedGithubProjects = useCallback(
    (t, providerId) =>
      [Providers.GITHUB].includes(providerId)
        ? [
            new GitHubProject({
              id: `${t.options?.owner}/${t.options?.repo}`,
              key: `${t.options?.owner}/${t.options?.repo}`,
              value: `${t.options?.owner}/${t.options?.repo}`,
              title: `${t.options?.owner}/${t.options?.repo}`
            })
          ]
        : [],
    []
  )

  const getAdvancedGitlabProjects = useCallback(
    (t, providerId) =>
      [Providers.GITLAB].includes(providerId)
        ? [
            new GitlabProject({
              id: t.options?.projectId,
              key: t.options?.projectId,
              value: t.options?.projectId,
              title: t.options?.title || `Project ${t.options?.projectId}`
            })
          ]
        : [],
    []
  )

  const getAdvancedJiraBoards = useCallback(
    (t, providerId) =>
      [Providers.JIRA].includes(providerId)
        ? [
            new JiraBoard({
              id: t.options?.boardId,
              key: t.options?.boardId,
              value: t.options?.boardId,
              title: t.options?.title || `Board ${t.options?.boardId}`
            })
          ]
        : [],
    []
  )

  // (altered version from PR No. 2926)
  // const getJiraMappedBoards = useCallback((options = []) => {
  //   return options.map(({ boardId, title }, sIdx) => {
  //     return {
  //       id: boardId,
  //       key: boardId,
  //       value: boardId,
  //       title: title || `Board ${boardId}`,
  //     }
  //   })
  // }, [])

  const getJiraMappedBoards = useCallback(
    (boardIds = [], boardListItems = []) => {
      return boardIds.map((bId, sIdx) => {
        const boardObject = boardListItems.find(
          (apiBoard) =>
            Number(apiBoard.id) === Number(bId) || +apiBoard.boardId === +bId
        )
        return new JiraBoard({
          ...boardObject,
          id: boardObject?.id || bId || sIdx + 1,
          key: sIdx,
          value: bId,
          title: boardObject?.name || boardObject?.title || `Board ${bId}`,
          type: boardObject?.type || 'scrum',
          location: { ...boardObject?.location }
        })
      })
    },
    []
  )

  const getDefaultEntities = useCallback((providerId) => {
    let entities = []
    switch (providerId) {
      case Providers.GITHUB:
      case Providers.GITLAB:
        entities = DEFAULT_DATA_ENTITIES
        break
      case Providers.JIRA:
        entities = DEFAULT_DATA_ENTITIES.filter(
          (d) => d.name === 'issue-tracking' || d.name === 'cross-domain'
        )
        break
      case Providers.JENKINS:
        entities = DEFAULT_DATA_ENTITIES.filter((d) => d.name === 'ci-cd')
        break
      case Providers.TAPD:
        entities = DEFAULT_DATA_ENTITIES.filter((d) => d.name === 'ci-cd')
        break
    }
    return entities
  }, [])

  const createNormalConnection = useCallback(
    (
      blueprint,
      c,
      cIdx,
      DEFAULT_DATA_ENTITIES,
      connections = [],
      connectionsList = [],
      boardsList = []
    ) => ({
      ...c,
      mode: BlueprintMode.NORMAL,
      // @IMPORTANT: Preserve Original LIST INDEX ID!
      id: connectionsList.find(
        (lC) => lC.value === c.connectionId && lC.provider === c.plugin
      )?.id,
      connectionId: c.connectionId,
      value: c.connectionId,
      provider: integrationsData.find((i) => i.id === c.plugin),
      providerLabel: ProviderLabels[c.plugin?.toUpperCase()],
      providerId: c.plugin,
      plugin: c.plugin,
      icon: ProviderIcons[c.plugin] ? ProviderIcons[c.plugin](18, 18) : null,
      name:
        connections.find(
          (pC) => pC.connectionId === c.connectionId && pC.plugin === c.plugin
        )?.name ||
        `${ProviderLabels[c.plugin?.toUpperCase()]} #${c.connectionId || cIdx}`,
      entities: c.scope[0]?.entities?.map(
        (e) => DEFAULT_DATA_ENTITIES.find((de) => de.value === e)?.title
      ),
      entityList: c.scope[0]?.entities?.map((e) =>
        DEFAULT_DATA_ENTITIES.find((de) => de.value === e)
      ),
      projects: [Providers.GITLAB].includes(c.plugin)
        ? getGitlabProjects(c)
        : getGithubProjects(c),
      boards: [Providers.JIRA].includes(c.plugin)
        ? c.scope.map((s) => `Board ${s.options?.boardId}`)
        : [],
      boardIds: [Providers.JIRA].includes(c.plugin)
        ? c.scope.map((s) => s.options?.boardId)
        : [],
      boardsList: boardsList,
      transformations: c.scope.map((s) => ({ ...s.transformation })),
      transformationStates: c.scope.map((s) =>
        Object.values(s.transformation).some((v) =>
          Array.isArray(v)
            ? v.length > 0
            : v && typeof v === 'object'
            ? Object.keys(v)?.length > 0
            : v?.toString().length > 0
        )
          ? 'Added'
          : '-'
      ),
      scope: c.scope,
      // editable: ![Providers.JENKINS].includes(c.plugin),
      editable: true,
      advancedEditable: false,
      isMultiStage: false,
      isSingleStage: true,
      stage: 1,
      totalStages: 1
    }),
    [getGithubProjects, getGitlabProjects]
  )

  const createAdvancedConnection = useCallback(
    (
      blueprint,
      c,
      cIdx,
      DEFAULT_DATA_ENTITIES,
      connections = [],
      connectionsList = [],
      boardsList = []
    ) => ({
      ...c,
      mode: BlueprintMode.ADVANCED,
      // @IMPORTANT: Preserve Original LIST INDEX ID!
      id: connectionsList.find(
        (lC) => lC.value === c.options?.connectionId && lC.provider === c.plugin
      )?.id,
      connectionId: c.options?.connectionId,
      value: c.options?.connectionId,
      provider: integrationsData.find((i) => i.id === c.plugin),
      providerLabel: ProviderLabels[c.plugin?.toUpperCase()],
      plugin: c.plugin,
      providerId: c.plugin,
      icon: ProviderIcons[c.plugin] ? ProviderIcons[c.plugin](18, 18) : null,
      name:
        connections.find(
          (pC) =>
            pC.connectionId === c.options?.connectionId &&
            pC.provider === c.plugin
        )?.name || `Connection ID #${c.options?.connectionId || cIdx}`,
      projects: [Providers.GITLAB].includes(c.plugin)
        ? getAdvancedGitlabProjects(c, c.plugin)
        : getAdvancedGithubProjects(c, c.plugin),
      entities: ['-'],
      entitityList: getDefaultEntities(c.plugin),
      boards: [Providers.JIRA].includes(c.plugin)
        ? getAdvancedJiraBoards(c, c.plugin).map((bId) => `Board ${bId}`)
        : [],
      boardIds: [Providers.JIRA].includes(c.plugin)
        ? getAdvancedJiraBoards(c, c.plugin)
        : [],
      boardsList: [Providers.JIRA].includes(c.plugin)
        ? getAdvancedJiraBoards(c, c.plugin).map((bId) => `Board ${bId}`)
        : [],
      transformations: [],
      transformationStates:
        typeof c.options?.transformationRules === 'object' &&
        Object.values(c.options?.transformationRules || {}).some(
          (v) => (Array.isArray(v) && v.length > 0) || v.toString().length > 0
        )
          ? ['Added']
          : ['-'],
      scope: c,
      task: c,
      editable: false,
      advancedEditable: true,
      plan: blueprint?.plan,
      isMultiStage:
        Array.isArray(blueprint?.plan) && blueprint?.plan.length > 1,
      isSingleStage:
        Array.isArray(blueprint?.plan) && blueprint?.plan.length === 1,
      stage:
        blueprint?.plan.findIndex((s, sId) =>
          s.find((t) => JSON.stringify(t) === JSON.stringify(c))
        ) + 1,
      totalStages: blueprint?.plan?.length
    }),
    [
      getAdvancedGithubProjects,
      getAdvancedGitlabProjects,
      getAdvancedJiraBoards,
      getDefaultEntities
    ]
  )

  useEffect(() => {
    console.log(
      '>>>>> DATA SCOPES MANAGER: INITIALIZING TRANSFORMATION RULES...',
      selectedProjects
    )
    console.log(
      '>>>>> DATA SCOPES MANAGER: INITIALIZING TRANSFORMATION RULES...',
      selectedBoards
    )
  }, [selectedProjects, selectedBoards, initializeTransformations])

  useEffect(() => {
    console.log('>>>>> DATA SCOPES MANAGER: CONFIGURED CONNECTION', connection)
    switch (connection?.provider?.id) {
      case Providers.GITHUB:
      case Providers.GITLAB:
        setProjects((p) => ({
          ...p,
          [connection?.id]: connection?.projects || []
        }))
        setEntities((e) => ({
          ...e,
          [connection?.id]: connection?.entityList || []
        }))
        connection?.projects.forEach((p, pIdx) =>
          setTransformationSettings(connection.transformations[pIdx], p?.id)
        )
        break
      case Providers.JIRA:
        // fetchBoards()
        // fetchIssueTypes()
        // fetchFields()
        setBoards((b) => ({
          ...b,
          [connection?.id]: connection?.boardsList || []
        }))
        setEntities((e) => ({
          ...e,
          [connection?.id]: connection?.entityList || []
        }))
        connection?.boardIds.forEach((bId, bIdx) =>
          setTransformationSettings(connection.transformations[bIdx], bId)
        )
        break
      case Providers.JENKINS:
        setEntities((e) => ({
          ...e,
          [connection?.id]: connection?.entityList || []
        }))
        setTransformationSettings(
          connection.transformations[0],
          `C#${connection?.id}`
        )
        break
    }
  }, [connection, setTransformationSettings])

  useEffect(() => {
    console.log('>>>>> DATA SCOPES MANAGER: Connection List...', connections)
    modifyConnectionSettings()
  }, [
    connections,
    entities,
    projects,
    boards,
    transformations,
    modifyConnectionSettings
  ])

  useEffect(() => {
    console.log('>>>>> DATA SCOPES MANAGER: PROVIDER...', provider)
    switch (provider?.id) {
      case Providers.GITHUB:
        break
      case Providers.GITLAB:
        break
      case Providers.JIRA:
        break
      case Providers.JENKINS:
        break
      case Providers.TAPD:
        break
    }
  }, [provider])

  useEffect(() => {
    console.log(
      '>>>>> DATA SCOPES MANAGER: INITIALIZE NEW CONNECTION TRANSFORMATIONS...',
      newConnections
    )
    // @note: jenkins has no "project/board" entity associated!
    // transformations are based on the main connection scope...
    const jenkinsTransformations = newConnections
      .filter((c) => c.plugin === Providers.JENKINS)
      .map((c) => `C#${c?.id}`)
    console.log(
      '>>>>> DATA SCOPES MANAGER: JENKINS TRANSFORMATIONS SCOPES...',
      jenkinsTransformations
    )
    if (Array.isArray(jenkinsTransformations)) {
      setTransformations((cT) => ({
        ...jenkinsTransformations.reduce(initializeTransformations, {}),
        // Spread Current/Existing Transformations Settings
        ...cT
      }))
    }
  }, [newConnections, initializeTransformations])

  useEffect(() => {
    console.log('>>>>> DATA SCOPES MANAGER: INITIALIZE BOARDS...', boards)
    const boardTransformations = boards[connection?.id]
    if (
      Array.isArray(boardTransformations) &&
      boardTransformations?.length > 0
    ) {
      setTransformations((cT) => ({
        ...boardTransformations.reduce(initializeTransformations, {}),
        // Spread Current/Existing Transformations Settings
        ...cT
      }))
    }
  }, [boards, connection?.id, initializeTransformations])

  useEffect(() => {
    console.log('>>>>> DATA SCOPES MANAGER: INITIALIZE PROJECTS...', projects)
    const projectTransformations = projects[connection?.id]
    if (Array.isArray(projectTransformations)) {
      setTransformations((cT) => ({
        ...projectTransformations.reduce(initializeTransformations, {}),
        // Spread Current/Existing Transformations Settings
        ...cT
      }))
    }
  }, [projects, connection?.id, initializeTransformations])

  useEffect(() => {
    console.log('>>>>> DATA SCOPES MANAGER: DATA ENTITIES...', entities)
  }, [entities])

  useEffect(() => {
    console.log('>>>>> DATA SCOPES MANAGER: DATA ENTITIES...', entities)
  }, [entities])

  useEffect(() => {
    console.log(
      '>>>>> DATA SCOPES MANAGER: TRANSFORMATIONS...',
      transformations
    )
  }, [transformations])

  useEffect(() => {
    console.log(
      '>>>>> DATA SCOPES MANAGER: CURRENT BLUEPRINT SETTINGS...',
      settings
    )
  }, [settings])

  useEffect(() => {
    console.log(
      '>>>>> DATA SCOPES MANAGER: ACTIVE TRANSFORMATION RULES...',
      activeTransformation
    )
  }, [activeTransformation])

  useEffect(() => {
    console.log(
      '>>>>> DATA SCOPES MANAGER: ACTIVE PROJECT TRANSFORMATION RULES...',
      activeProjectTransformation
    )
  }, [activeProjectTransformation])

  useEffect(() => {
    console.log(
      '>>>>> DATA SCOPES MANAGER: ACTIVE BOARD TRANSFORMATION RULES...',
      activeBoardTransformation
    )
  }, [activeBoardTransformation])

  useEffect(() => {
    console.log(
      '>>>>> DATA SCOPES MANAGER: MEMOIZED ACTIVE CONNECTION...',
      connection
    )
  }, [connection])

  return {
    connections,
    newConnections,
    // blueprint,
    boards,
    projects,
    entities,
    transformations,
    configuredConnection,
    configuredBoard,
    configuredProject,
    configurationKey,
    storedProjectTransformation,
    storedBoardTransformation,
    activeBoardTransformation,
    activeProjectTransformation,
    activeTransformation,
    scopeConnection,
    enabledProviders,
    // setActiveTransformation,
    setNewConnections,
    setConnections,
    setScopeConnection,
    setConfiguredConnection,
    setConfiguredBoard,
    setConfiguredProject,
    // setBlueprint,
    setBoards,
    setProjects,
    setEntities,
    setTransformations,
    setTransformationSettings,
    initializeTransformations,
    getDefaultTransformations,
    createProviderConnections,
    createProviderScopes,
    getJiraMappedBoards,
    getDefaultEntities,
    createNormalConnection,
    createAdvancedConnection,
    modifyConnectionSettings,
    setEnabledProviders
  }
}

export default useDataScopesManager
