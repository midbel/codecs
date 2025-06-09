<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<language>
			<xsl:for-each select="/root/programming">
				<lang><xsl:value-of select="./@id"/></lang>
				<xsl:on-empty>
					<p>no programming languages</p>
				</xsl:on-empty>
			</xsl:for-each>
		</language>
	</xsl:template>
</xsl:stylesheet>