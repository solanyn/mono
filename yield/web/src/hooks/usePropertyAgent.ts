import { useCopilotAction } from '@copilotkit/react-core'
import { useNavigate } from 'react-router'

export function usePropertyAgent() {
  const navigate = useNavigate()

  useCopilotAction({
    name: 'navigateToSuburb',
    description: 'Navigate to a suburb overview page',
    parameters: [
      { name: 'suburb', type: 'string', description: 'Suburb name', required: true },
    ],
    handler: ({ suburb }) => {
      const slug = suburb.toLowerCase().replace(/\s+/g, '-')
      navigate(`/suburb/${slug}`)
    },
  })

  useCopilotAction({
    name: 'navigateToProperty',
    description: 'Navigate to a property detail page',
    parameters: [
      { name: 'propertyId', type: 'string', description: 'Property ID', required: true },
    ],
    handler: ({ propertyId }) => {
      navigate(`/property/${propertyId}`)
    },
  })

  useCopilotAction({
    name: 'navigateToRentCheck',
    description: 'Navigate to the rent check page',
    parameters: [],
    handler: () => {
      navigate('/rent-check')
    },
  })

  useCopilotAction({
    name: 'navigateToAnalyze',
    description: 'Navigate to the property analysis page',
    parameters: [],
    handler: () => {
      navigate('/analyze')
    },
  })

  useCopilotAction({
    name: 'navigateToSearch',
    description: 'Navigate to the search page',
    parameters: [],
    handler: () => {
      navigate('/search')
    },
  })

  useCopilotAction({
    name: 'navigateToPortfolio',
    description: 'Navigate to the portfolio page',
    parameters: [],
    handler: () => {
      navigate('/portfolio')
    },
  })
}
